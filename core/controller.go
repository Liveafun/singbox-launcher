package core

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/api"
	"singbox-launcher/internal/constants"
	"singbox-launcher/internal/platform"

	ps "github.com/mitchellh/go-ps"
)

// Constants for log file names
const (
	logFileName             = "logs/" + constants.MainLogFileName
	childLogFileName        = "logs/" + constants.ChildLogFileName
	parserLogFileName       = "logs/" + constants.ParserLogFileName
	apiLogFileName          = "logs/" + constants.APILogFileName
	restartAttempts         = 3
	restartDelay            = 2 * time.Second
	stabilityThreshold      = 180 * time.Second
	gracefulShutdownTimeout = 2 * time.Second
	maxLogFileSize          = 10 * 1024 * 1024 // 10 MB - maximum log file size before rotation
)

// AppController - the main structure encapsulating all application state and logic.
type AppController struct {
	// --- Fyne Components ---
	Application    fyne.App
	MainWindow     fyne.Window
	TrayIcon       fyne.Resource
	ApiStatusLabel *widget.Label

	// --- UI State Fields ---
	ProxiesListWidget *widget.List
	ActiveProxyName   string
	SelectedIndex     int
	ProxiesList       []api.ProxyInfo
	ListStatusLabel   *widget.Label

	// --- Icon Resources ---
	AppIconData   fyne.Resource
	GreenIconData fyne.Resource
	GreyIconData  fyne.Resource
	RedIconData   fyne.Resource // Иконка для состояния ошибки

	// --- Process State ---
	SingboxCmd               *exec.Cmd
	CmdMutex                 sync.Mutex
	ParserMutex              sync.Mutex // Mutex for ParserRunning
	ParserRunning            bool
	StoppedByUser            bool
	ConsecutiveCrashAttempts int
	APIStateMutex            sync.RWMutex // Mutex for API-related fields (ProxiesList, ActiveProxyName, SelectedIndex)

	// --- File Paths ---
	ExecDir     string
	ConfigPath  string
	SingboxPath string
	ParserPath  string
	WintunPath  string

	// --- VPN Operation State ---
	RunningState *RunningState

	// --- Logging ---
	MainLogFile  *os.File
	ChildLogFile *os.File
	ApiLogFile   *os.File

	// --- Clash API configuration ---
	ClashAPIBaseURL    string
	ClashAPIToken      string
	ClashAPIEnabled    bool
	SelectedClashGroup string
	AutoLoadInProgress bool       // Flag to prevent multiple auto-load attempts
	AutoLoadMutex      sync.Mutex // Mutex for AutoLoadInProgress

	// --- Callbacks for UI logic ---
	RefreshAPIFunc       func()
	ResetAPIStateFunc    func()
	UpdateCoreStatusFunc func() // Callback для обновления статуса в Core Dashboard
	UpdateTrayMenuFunc   func() // Callback для обновления меню трея
}

// RunningState - structure for tracking the VPN's running state.
type RunningState struct {
	running bool
	sync.Mutex
	controller *AppController
}

// checkAndRotateLogFile checks log file size and rotates if it exceeds maxLogFileSize
func checkAndRotateLogFile(logPath string) {
	info, err := os.Stat(logPath)
	if err != nil {
		return // File doesn't exist yet, nothing to rotate
	}

	if info.Size() > maxLogFileSize {
		// Rotate: rename current file to .old
		oldPath := logPath + ".old"
		_ = os.Remove(oldPath) // Remove old backup if exists
		if err := os.Rename(logPath, oldPath); err != nil {
			log.Printf("checkAndRotateLogFile: Failed to rotate log file %s: %v", logPath, err)
		} else {
			log.Printf("checkAndRotateLogFile: Rotated log file %s (size: %d bytes)", logPath, info.Size())
		}
	}
}

// openLogFileWithRotation opens a log file and rotates it if it exceeds maxLogFileSize
func openLogFileWithRotation(logPath string) (*os.File, error) {
	checkAndRotateLogFile(logPath)

	// Open file in append mode (not truncate) to preserve recent logs
	// But if file was rotated, it will be a new file
	return os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

// NewAppController creates and initializes a new AppController instance.
func NewAppController(appIconData, greyIconData, greenIconData, redIconData []byte) (*AppController, error) {
	ac := &AppController{}

	ex, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("NewAppController: cannot determine executable path: %w", err)
	}
	ac.ExecDir = filepath.Dir(ex)

	// Use platform-specific functions
	if err := platform.EnsureDirectories(ac.ExecDir); err != nil {
		return nil, fmt.Errorf("NewAppController: cannot create directories: %w", err)
	}

	ac.ConfigPath = platform.GetConfigPath(ac.ExecDir)
	singboxName, parserName := platform.GetExecutableNames()
	ac.SingboxPath = filepath.Join(ac.ExecDir, "bin", singboxName)
	ac.ParserPath = filepath.Join(ac.ExecDir, "bin", parserName)
	ac.WintunPath = platform.GetWintunPath(ac.ExecDir)

	// Open log files with rotation support
	logFile, err := openLogFileWithRotation(filepath.Join(ac.ExecDir, logFileName))
	if err != nil {
		return nil, fmt.Errorf("NewAppController: cannot open main log file: %w", err)
	}
	log.SetOutput(logFile)
	ac.MainLogFile = logFile

	childLogFile, err := openLogFileWithRotation(filepath.Join(ac.ExecDir, childLogFileName))
	if err != nil {
		log.Printf("NewAppController: failed to open sing-box child log file: %v", err)
		ac.ChildLogFile = nil
	} else {
		ac.ChildLogFile = childLogFile
	}

	apiLogFile, err := openLogFileWithRotation(filepath.Join(ac.ExecDir, apiLogFileName))
	if err != nil {
		log.Printf("NewAppController: failed to open API log file: %v", err)
		ac.ApiLogFile = nil
	} else {
		ac.ApiLogFile = apiLogFile
	}

	ac.AppIconData = fyne.NewStaticResource("appIcon", appIconData)
	ac.GreyIconData = fyne.NewStaticResource("trayIcon", greyIconData)
	ac.GreenIconData = fyne.NewStaticResource("runningIcon", greenIconData)
	ac.RedIconData = fyne.NewStaticResource("errorIcon", redIconData)

	log.Println("Application initializing...")
	ac.Application = app.NewWithID("com.singbox.launcher")
	ac.Application.SetIcon(ac.AppIconData)
	ac.RunningState = &RunningState{controller: ac}
	ac.RunningState.Set(false) // Use Set() method instead of direct assignment
	ac.ConsecutiveCrashAttempts = 0

	if base, tok, err := api.LoadClashAPIConfig(ac.ConfigPath); err != nil {
		log.Printf("NewAppController: Clash API config error: %v", err)
		ac.ClashAPIBaseURL = ""
		ac.ClashAPIToken = ""
		ac.ClashAPIEnabled = false
	} else {
		ac.ClashAPIBaseURL = base
		ac.ClashAPIToken = tok
		ac.ClashAPIEnabled = true
	}

	// Initialize SelectedClashGroup from config (needed for auto-loading proxies)
	if ac.ClashAPIEnabled {
		_, defaultSelector, err := GetSelectorGroupsFromConfig(ac.ConfigPath)
		if err != nil {
			log.Printf("NewAppController: Failed to get selector groups: %v", err)
			ac.SelectedClashGroup = "proxy-out" // Default fallback
		} else {
			ac.SelectedClashGroup = defaultSelector
			log.Printf("NewAppController: Initialized SelectedClashGroup: %s", defaultSelector)
		}
	}

	// Initialize API state fields (safe during initialization, but using methods for consistency)
	ac.SetProxiesList([]api.ProxyInfo{})
	ac.SetSelectedIndex(-1)
	ac.SetActiveProxyName("")

	ac.RefreshAPIFunc = func() { log.Println("RefreshAPIFunc handler is not set yet.") }
	ac.ResetAPIStateFunc = func() { log.Println("ResetAPIStateFunc handler is not set yet.") }
	ac.UpdateCoreStatusFunc = func() { log.Println("UpdateCoreStatusFunc handler is not set yet.") }
	ac.UpdateTrayMenuFunc = func() { log.Println("UpdateTrayMenuFunc handler is not set yet.") }

	return ac, nil
}

// UpdateUI updates all UI elements based on the current application state.
func (ac *AppController) UpdateUI() {
	fyne.Do(func() {
		// Обновляем иконку трея (это системная функция, не UI виджет)
		if desk, ok := ac.Application.(desktop.App); ok {
			// Проверяем, что иконки инициализированы
			if ac.GreenIconData == nil || ac.GreyIconData == nil || ac.RedIconData == nil {
				log.Printf("UpdateUI: Icons not initialized, skipping icon update")
				return
			}

			var iconToSet fyne.Resource

			if ac.RunningState.IsRunning() {
				// Зеленая иконка - если запущен
				iconToSet = ac.GreenIconData
			} else {
				// Проверяем наличие бинарника для определения ошибки (простая проверка файла)
				if _, err := os.Stat(ac.SingboxPath); os.IsNotExist(err) {
					// Красная иконка - при ошибке (бинарник не найден)
					iconToSet = ac.RedIconData
				} else {
					// Черная иконка - при штатной остановке
					iconToSet = ac.GreyIconData
				}
			}

			if iconToSet != nil {
				desk.SetSystemTrayIcon(iconToSet)
			}
		}

		// Если состояние Down, сбрасываем API состояние
		if !ac.RunningState.IsRunning() && ac.ResetAPIStateFunc != nil {
			log.Println("UpdateUI: Triggering API state reset because state is 'Down'.")
			ac.ResetAPIStateFunc()
		}
	})
}

// GracefulExit performs a graceful shutdown of the application.
func (ac *AppController) GracefulExit() {
	ac.StopSingBox()

	log.Println("GracefulExit: Waiting for sing-box to stop...")
	timeout := time.After(gracefulShutdownTimeout)
	for {
		if !ac.RunningState.IsRunning() {
			log.Println("GracefulExit: Sing-box confirmed stopped.")
			break
		}
		select {
		case <-timeout:
			log.Println("GracefulExit: Timeout waiting for sing-box to stop. Forcing kill.")
			ac.CmdMutex.Lock()
			if ac.SingboxCmd != nil && ac.SingboxCmd.Process != nil {
				_ = ac.SingboxCmd.Process.Kill()
			}
			ac.CmdMutex.Unlock()
			goto end_loop
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
end_loop:

	if ac.MainLogFile != nil {
		ac.MainLogFile.Close()
	}
	if ac.ChildLogFile != nil {
		ac.ChildLogFile.Close()
	}
	if ac.ApiLogFile != nil {
		ac.ApiLogFile.Close()
	}

	ac.Application.Quit()
}

// StartSingBox launches the sing-box process.
func (ac *AppController) StartSingBox() {
	StartSingBoxProcess(ac)
}

// StopSingBox stops the sing-box process.
func (ac *AppController) StopSingBox() {
	StopSingBoxProcess(ac)
}

// RunParser launches the parser process.
func (ac *AppController) RunParser() {
	RunParserProcess(ac)
}

// RunHidden launches an external command in a hidden window.
func (ac *AppController) RunHidden(name string, args []string, logPath string, dir string) error {
	cmd := exec.Command(name, args...)
	platform.PrepareCommand(cmd)
	if dir != "" {
		cmd.Dir = dir
	}

	if logPath != "" {
		if logPath == filepath.Join(ac.ExecDir, childLogFileName) && ac.ChildLogFile != nil {
			// For sing-box logs, check and rotate if needed before writing
			checkAndRotateLogFile(logPath)
			logFile := ac.ChildLogFile
			// Don't truncate - append to preserve logs, rotation handles size limits
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		} else {
			// For other logs (parser), use truncate mode for clean start
			logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return fmt.Errorf("RunHidden: cannot open log file '%s': %w", logPath, err)
			}
			defer logFile.Close()
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}
	}

	return cmd.Run()
}

// ShowAutoHideInfo displays a temporary Fyne dialog and a system notification simultaneously.
func (ac *AppController) ShowAutoHideInfo(title, message string) {
	ShowAutoHideInfoUtil(ac, title, message)
}

// ShowErrorDialog displays an error dialog in a thread-safe way.
func (ac *AppController) ShowErrorDialog(err error) {
	fyne.Do(func() {
		dialog.ShowError(err, ac.MainWindow)
	})
}

// ShowSingBoxAlreadyRunningWarning displays a warning if sing-box is already running.
func (ac *AppController) ShowSingBoxAlreadyRunningWarning() {
	ShowSingBoxAlreadyRunningWarningUtil(ac)
}

// CheckFiles checks for the presence of necessary files and displays the result.
func (ac *AppController) CheckFiles() {
	CheckFilesUtil(ac)
}

// CheckLinuxCapabilities checks Linux capabilities and shows a suggestion if needed
func CheckLinuxCapabilities(ac *AppController) {
	if suggestion := platform.CheckAndSuggestCapabilities(ac.SingboxPath); suggestion != "" {
		log.Printf("CheckLinuxCapabilities: %s", suggestion)
		// Show info dialog (not error) - capabilities can be set later
		fyne.Do(func() {
			dialog.ShowInformation(
				"Linux Capabilities",
				suggestion,
				ac.MainWindow,
			)
		})
	}
}

// Set sets the new value for the 'running' state and triggers a UI update.
func (r *RunningState) Set(value bool) {
	r.Lock()
	if r.running == value {
		r.Unlock()
		return
	}
	r.running = value
	r.Unlock()

	r.controller.UpdateUI()

	// Вызываем callback для обновления статуса в Core Dashboard
	if r.controller.UpdateCoreStatusFunc != nil {
		r.controller.UpdateCoreStatusFunc()
	}

}

// IsRunning checks if the VPN is running.
func (r *RunningState) IsRunning() bool {
	r.Lock()
	defer r.Unlock()
	return r.running
}

// SetProxiesList safely sets the proxies list with mutex protection.
func (ac *AppController) SetProxiesList(proxies []api.ProxyInfo) {
	ac.APIStateMutex.Lock()
	defer ac.APIStateMutex.Unlock()
	ac.ProxiesList = proxies
}

// GetProxiesList safely gets a copy of the proxies list with mutex protection.
func (ac *AppController) GetProxiesList() []api.ProxyInfo {
	ac.APIStateMutex.RLock()
	defer ac.APIStateMutex.RUnlock()
	// Return a copy to prevent external modifications
	result := make([]api.ProxyInfo, len(ac.ProxiesList))
	copy(result, ac.ProxiesList)
	return result
}

// SetActiveProxyName safely sets the active proxy name with mutex protection.
func (ac *AppController) SetActiveProxyName(name string) {
	ac.APIStateMutex.Lock()
	defer ac.APIStateMutex.Unlock()
	ac.ActiveProxyName = name
}

// GetActiveProxyName safely gets the active proxy name with mutex protection.
func (ac *AppController) GetActiveProxyName() string {
	ac.APIStateMutex.RLock()
	defer ac.APIStateMutex.RUnlock()
	return ac.ActiveProxyName
}

// SetSelectedIndex safely sets the selected index with mutex protection.
func (ac *AppController) SetSelectedIndex(index int) {
	ac.APIStateMutex.Lock()
	defer ac.APIStateMutex.Unlock()
	ac.SelectedIndex = index
}

// GetSelectedIndex safely gets the selected index with mutex protection.
func (ac *AppController) GetSelectedIndex() int {
	ac.APIStateMutex.RLock()
	defer ac.APIStateMutex.RUnlock()
	return ac.SelectedIndex
}

// isSingBoxProcessRunning checks if a sing-box process is currently running on the system.
// Uses tasklist command on Windows for more reliable process detection.
// Returns true if process found, and the PID of found process (or -1 if not found).
func isSingBoxProcessRunning(ac *AppController) (bool, int) {
	processName := platform.GetProcessNameForCheck()
	log.Printf("isSingBoxProcessRunning: Looking for process name '%s'", processName)

	ac.CmdMutex.Lock()
	ourPID := -1
	if ac.SingboxCmd != nil && ac.SingboxCmd.Process != nil {
		ourPID = ac.SingboxCmd.Process.Pid
	}
	ac.CmdMutex.Unlock()
	log.Printf("isSingBoxProcessRunning: Our tracked PID=%d", ourPID)

	// На Windows используем tasklist для более надежной проверки
	if runtime.GOOS == "windows" {
		// Используем tasklist /FI "IMAGENAME eq sing-box.exe" /FO CSV /NH
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", processName), "/FO", "CSV", "/NH")
		platform.PrepareCommand(cmd) // Скрываем консольное окно
		output, err := cmd.Output()
		if err != nil {
			log.Printf("isSingBoxProcessRunning: tasklist command failed: %v, falling back to ps library", err)
			// Fallback на старый способ
			return isSingBoxProcessRunningFallback(ac)
		}

		// Парсим CSV вывод tasklist
		// Формат: "имя.exe","PID","Session Name","Session#","Mem Usage"
		outputStr := strings.TrimSpace(string(output))
		if outputStr == "" {
			log.Printf("isSingBoxProcessRunning: No sing-box process found via tasklist")
			return false, -1
		}

		// Парсим CSV строку
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Парсим CSV: "имя.exe","PID","..."
			// Используем более надежный парсинг CSV
			parts := parseCSVLine(line)
			if len(parts) >= 2 {
				name := strings.Trim(parts[0], "\"")
				pidStr := strings.Trim(parts[1], "\"")
				if strings.EqualFold(name, processName) {
					if pid, err := strconv.Atoi(pidStr); err == nil {
						isOurProcess := (ourPID != -1 && pid == ourPID)
						log.Printf("isSingBoxProcessRunning: Found process via tasklist: PID=%d, name='%s' (our tracked PID=%d, isOurProcess=%v)", pid, name, ourPID, isOurProcess)
						return true, pid
					} else {
						log.Printf("isSingBoxProcessRunning: Failed to parse PID '%s': %v", pidStr, err)
					}
				}
			}
		}
		log.Printf("isSingBoxProcessRunning: tasklist found processes but none matched '%s'", processName)
		return false, -1
	}

	// Для других ОС используем старый способ
	return isSingBoxProcessRunningFallback(ac)
}

// parseCSVLine парсит CSV строку, учитывая кавычки
func parseCSVLine(line string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false

	for i, r := range line {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
		// Последний символ
		if i == len(line)-1 {
			parts = append(parts, current.String())
		}
	}

	return parts
}

// isSingBoxProcessRunningFallback uses ps library as fallback method
func isSingBoxProcessRunningFallback(ac *AppController) (bool, int) {
	processes, err := ps.Processes()
	if err != nil {
		log.Printf("isSingBoxProcessRunningFallback: error listing processes: %v", err)
		return false, -1
	}
	processName := platform.GetProcessNameForCheck()

	ac.CmdMutex.Lock()
	ourPID := -1
	if ac.SingboxCmd != nil && ac.SingboxCmd.Process != nil {
		ourPID = ac.SingboxCmd.Process.Pid
	}
	ac.CmdMutex.Unlock()

	for _, p := range processes {
		execName := p.Executable()
		if strings.EqualFold(execName, processName) {
			foundPID := p.Pid()
			isOurProcess := (ourPID != -1 && foundPID == ourPID)
			log.Printf("isSingBoxProcessRunningFallback: Found process: PID=%d, executable='%s' (our tracked PID=%d, isOurProcess=%v)", foundPID, execName, ourPID, isOurProcess)
			return true, foundPID
		}
	}
	log.Printf("isSingBoxProcessRunningFallback: No sing-box process found (checked %d processes)", len(processes))
	return false, -1
}

// checkAndShowSingBoxRunningWarning checks if sing-box is running and shows warning dialog if found.
// Returns true if process was found and warning was shown, false otherwise.
func checkAndShowSingBoxRunningWarning(ac *AppController, context string) bool {
	found, foundPID := isSingBoxProcessRunning(ac)
	if found {
		log.Printf("%s: Found sing-box process already running (PID=%d). Showing warning dialog.", context, foundPID)
		ac.ShowSingBoxAlreadyRunningWarning()
		return true
	}
	log.Printf("%s: No sing-box process found", context)
	return false
}

// StartSingBoxProcess launches the sing-box process.
func StartSingBoxProcess(ac *AppController) {
	if ac.RunningState.IsRunning() {
		ac.ShowAutoHideInfo("Info", "Sing-Box already running (according to internal state).")
		return
	}

	// Проверяем, не запущен ли уже процесс на уровне ОС
	if checkAndShowSingBoxRunningWarning(ac, "startSingBox") {
		return
	}

	ac.CmdMutex.Lock()
	defer ac.CmdMutex.Unlock()

	// Check capabilities on Linux before starting
	if suggestion := platform.CheckAndSuggestCapabilities(ac.SingboxPath); suggestion != "" {
		log.Printf("startSingBox: Capabilities check failed: %s", suggestion)
		ac.ShowErrorDialog(fmt.Errorf("Linux capabilities required\n\n%s", suggestion))
		return
	}

	log.Println("startSingBox: Starting Sing-Box...")
	ac.SingboxCmd = exec.Command(ac.SingboxPath, "run", "-c", filepath.Base(ac.ConfigPath))
	platform.PrepareCommand(ac.SingboxCmd)
	ac.SingboxCmd.Dir = platform.GetBinDir(ac.ExecDir)
	if ac.ChildLogFile != nil {
		// Check and rotate log file before starting new process to prevent unbounded growth
		checkAndRotateLogFile(filepath.Join(ac.ExecDir, childLogFileName))

		// Write directly to file - no buffering in memory
		// This prevents memory leaks from accumulating log output
		// Logs are written immediately to disk, not stored in memory
		ac.SingboxCmd.Stdout = ac.ChildLogFile
		ac.SingboxCmd.Stderr = ac.ChildLogFile
	} else {
		log.Println("startSingBox: Warning: sing-box log file not available, output will not be logged.")
	}
	if err := ac.SingboxCmd.Start(); err != nil {
		ac.ShowStartupError(fmt.Errorf("failed to start Sing-Box process: %w", err))
		log.Printf("startSingBox: Failed to start Sing-Box: %v", err)
		return
	}
	ac.RunningState.Set(true)
	ac.StoppedByUser = false
	//Добавляем лог с PID
	log.Printf("startSingBox: Sing-Box started. PID=%d", ac.SingboxCmd.Process.Pid)

	go MonitorSingBoxProcess(ac, ac.SingboxCmd)
}

// MonitorSingBoxProcess monitors the sing-box process.
func MonitorSingBoxProcess(ac *AppController, cmdToMonitor *exec.Cmd) {
	// Store the PID we're monitoring to avoid conflicts with restarted processes
	monitoredPID := cmdToMonitor.Process.Pid

	// Wait for process completion - no timeout for long-running processes
	// The process should run until it exits or is stopped by user
	err := cmdToMonitor.Wait()

	ac.CmdMutex.Lock()
	defer ac.CmdMutex.Unlock()

	// ЗОЛОТОЙ СТАНДАРТ: Порядок проверок для защиты от всех гонок
	// 1. Сначала PID (не мой ли процесс?)
	if ac.SingboxCmd == nil || ac.SingboxCmd.Process == nil || ac.SingboxCmd.Process.Pid != monitoredPID {
		log.Printf("monitorSingBox: Process was restarted (PID changed from %d). This monitor is obsolete. Exiting.", monitoredPID)
		return
	}

	// 2. Потом StoppedByUser (пользователь остановил?)
	if ac.StoppedByUser {
		log.Println("monitorSingBox: Sing-Box exited as requested by user.")
		ac.ConsecutiveCrashAttempts = 0
		ac.RunningState.Set(false)
		ac.StoppedByUser = false // Сбрасываем флаг для следующего запуска
		return
	}

	// 3. Потом err == nil (нормально вышел?)
	if err == nil {
		log.Println("monitorSingBox: Sing-Box exited gracefully (exit code 0).")
		ac.ConsecutiveCrashAttempts = 0
		ac.RunningState.Set(false)
		return
	}

	// 4. Только потом — краш → рестарт
	// Процесс завершился с ошибкой - проверяем лимит попыток
	ac.RunningState.Set(false)
	ac.ConsecutiveCrashAttempts++

	if ac.ConsecutiveCrashAttempts > restartAttempts {
		log.Printf("monitorSingBox: Maximum restart attempts (%d) reached. Stopping auto-restart.", restartAttempts)
		ac.ShowErrorDialog(fmt.Errorf("Sing-Box failed to restart after %d attempts. Check sing-box.log for details.", restartAttempts))
		ac.ConsecutiveCrashAttempts = 0
		return
	}

	// Пытаемся перезапустить
	log.Printf("monitorSingBox: Sing-Box crashed: %v, attempting auto-restart (attempt %d/%d)", err, ac.ConsecutiveCrashAttempts, restartAttempts)
	ac.ShowAutoHideInfo("Crash", fmt.Sprintf("Sing-Box crashed, restarting... (attempt %d/%d)", ac.ConsecutiveCrashAttempts, restartAttempts))

	ac.CmdMutex.Unlock()
	StartSingBoxProcess(ac)
	ac.CmdMutex.Lock()

	if ac.RunningState.IsRunning() {
		log.Println("monitorSingBox: Sing-Box restarted successfully.")
		currentAttemptCount := ac.ConsecutiveCrashAttempts
		go func() {
			time.Sleep(stabilityThreshold)
			ac.CmdMutex.Lock()
			defer ac.CmdMutex.Unlock()

			if ac.RunningState.IsRunning() && ac.ConsecutiveCrashAttempts == currentAttemptCount {
				log.Printf("monitorSingBox: Process has been stable for %v. Resetting crash counter from %d to 0.", stabilityThreshold, ac.ConsecutiveCrashAttempts)
				ac.ConsecutiveCrashAttempts = 0
			} else {
				log.Printf("monitorSingBox: Stability timer expired, but conditions for reset not met (running: %v, current attempts: %d, attempts at timer start: %d).", ac.RunningState.IsRunning(), ac.ConsecutiveCrashAttempts, currentAttemptCount)
			}
		}()
	} else {
		log.Printf("monitorSingBox: Restart attempt %d failed.", ac.ConsecutiveCrashAttempts)
	}
}

// StopSingBoxProcess is the unified function to stop the sing-box process.
func StopSingBoxProcess(ac *AppController) {
	ac.CmdMutex.Lock()

	// КРИТИЧНО: Устанавливаем флаг ПЕРЕД отправкой сигнала
	// Это гарантирует, что монитор увидит флаг, даже если процесс завершится очень быстро
	ac.StoppedByUser = true
	ac.ConsecutiveCrashAttempts = 0

	if !ac.RunningState.IsRunning() {
		ac.StoppedByUser = false
		ac.CmdMutex.Unlock()
		return
	}

	if ac.SingboxCmd == nil || ac.SingboxCmd.Process == nil {
		log.Println("StopSingBoxProcess: Inconsistent state detected. Correcting state.")
		ac.RunningState.Set(false)
		ac.StoppedByUser = false
		ac.CmdMutex.Unlock()
		return
	}

	log.Println("stopSingBox: Attempting graceful shutdown...")
	processToStop := ac.SingboxCmd.Process

	// Разблокируем мьютекс перед отправкой сигнала, чтобы не блокировать
	ac.CmdMutex.Unlock()

	var err error
	if runtime.GOOS == "windows" {
		// sing-box ловит именно CTRL_BREAK_EVENT на Windows
		dll := syscall.NewLazyDLL("kernel32.dll")
		proc := dll.NewProc("GenerateConsoleCtrlEvent")
		if r, _, e := proc.Call(uintptr(syscall.CTRL_BREAK_EVENT), uintptr(processToStop.Pid)); r == 0 {
			err = e
		}
	} else {
		err = processToStop.Signal(os.Interrupt)
	}

	if err != nil {
		log.Printf("stopSingBox: Graceful signal failed: %v. Forcing kill.", err)
		if killErr := processToStop.Kill(); killErr != nil {
			log.Printf("stopSingBox: Failed to kill Sing-Box process: %v", killErr)
		}
	} else {
		// ИЗМЕНЕНО: Запускаем "сторожевой" таймер, который убьет процесс, если он не закроется сам
		log.Println("stopSingBox: Signal sent, starting watchdog timer...")
		go func(pid int) {
			time.Sleep(gracefulShutdownTimeout)
			p, _ := ps.FindProcess(pid)
			if p != nil {
				log.Printf("stopSingBox watchdog: Process %d still running after timeout. Forcing kill.", pid)
				// Надёжно убиваем процесс и его дочерние процессы
				_ = platform.KillProcessByPID(pid)
			}
		}(processToStop.Pid)
	}
}

// RunParserProcess запускает встроенный процесс обновления конфига.
func RunParserProcess(ac *AppController) {
	// Проверяем, не запущен ли уже парсинг
	ac.ParserMutex.Lock()
	if ac.ParserRunning {
		ac.ParserMutex.Unlock()
		ac.ShowAutoHideInfo("Parser Info", "Configuration update is already in progress.")
		return
	}
	ac.ParserRunning = true
	ac.ParserMutex.Unlock()

	log.Println("RunParser: Starting internal configuration update...")
	// Гарантируем, что флаг сбросится после завершения, даже если будет ошибка
	defer func() {
		ac.ParserMutex.Lock()
		ac.ParserRunning = false
		ac.ParserMutex.Unlock()
	}()

	// Вызываем встроенный парсер для обновления конфигурации
	err := UpdateConfigFromSubscriptions(ac)

	// Обрабатываем результат
	if err != nil {
		log.Printf("RunParser: Failed to update config: %v", err)
		ac.ShowParserError(fmt.Errorf("failed to update config: %w", err))
	} else {
		log.Println("RunParser: Config updated successfully.")
		ac.ShowAutoHideInfo("Parser", "Config updated successfully!")
	}
}

func CheckIfSingBoxRunningAtStartUtil(ac *AppController) {
	checkAndShowSingBoxRunningWarning(ac, "CheckIfSingBoxRunningAtStart")
}

// CheckConfigFileExists checks if config.json exists and shows a warning if it doesn't
func CheckConfigFileExists(ac *AppController) {
	if _, err := os.Stat(ac.ConfigPath); os.IsNotExist(err) {
		log.Printf("CheckConfigFileExists: config.json not found at %s", ac.ConfigPath)
		examplePath := filepath.Join(platform.GetBinDir(ac.ExecDir), constants.ConfigExampleName)

		message := fmt.Sprintf(
			"⚠️ Configuration file not found!\n\n"+
				"The file %s is missing from the bin/ folder.\n\n"+
				"To get started:\n"+
				"1. Copy the file %s to %s\n"+
				"2. Open %s and fill it with your settings\n"+
				"3. Restart the application\n\n"+
				"Example configuration is located here:\n%s",
			constants.ConfigFileName,
			constants.ConfigExampleName,
			constants.ConfigFileName,
			constants.ConfigFileName,
			examplePath,
		)

		fyne.Do(func() {
			dialog.ShowInformation("Configuration Not Found", message, ac.MainWindow)
		})
	}
}

func CheckIfLauncherAlreadyRunningUtil(ac *AppController) {
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("CheckIfLauncherAlreadyRunning: cannot detect executable path: %v", err)
		return
	}
	execName := strings.ToLower(filepath.Base(execPath))
	currentPID := os.Getpid()

	processes, err := ps.Processes()
	if err != nil {
		log.Printf("CheckIfLauncherAlreadyRunning: error listing processes: %v", err)
		return
	}

	for _, p := range processes {
		if p.Pid() == currentPID {
			continue
		}
		if strings.EqualFold(p.Executable(), execName) {
			fyne.Do(func() {
				dialog.ShowInformation(
					"Information",
					"The application is already running. Use the existing instance or close it before starting a new one.",
					ac.MainWindow,
				)
			})
			return
		}
	}
}

func CheckFilesUtil(ac *AppController) {
	files := platform.GetRequiredFiles(ac.ExecDir)
	msg := "File check:\n\n"
	allOk := true
	for _, f := range files {
		info, err := os.Stat(f.Path)
		if err == nil {
			size := FormatBytesUtil(info.Size())
			msg += fmt.Sprintf("%s (%s): Found (%s)\n", f.Name, f.Path, size)
		} else {
			msg += fmt.Sprintf("%s (%s): Not Found (Error: %v)\n", f.Name, f.Path, err)
			allOk = false
		}
	}
	if allOk {
		msg += "\nAll files found. ✅"
	} else {
		msg += "\nSome files missing. ❌"
	}
	dialog.ShowInformation("File Check", msg, ac.MainWindow)
}

func FormatBytesUtil(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func ShowSingBoxAlreadyRunningWarningUtil(ac *AppController) {
	label := widget.NewLabel("Sing-Box appears to be already running.\nWould you like to kill the existing process?")
	killButton := widget.NewButton("Kill Process", nil)
	closeButton := widget.NewButton("Close This Warning", nil)
	content := container.NewVBox(label, killButton, closeButton)
	var d dialog.Dialog
	d = dialog.NewCustomWithoutButtons("Warning", content, ac.MainWindow)
	killButton.OnTapped = func() {
		go func() {
			processName := platform.GetProcessNameForCheck()
			_ = platform.KillProcess(processName)
			ac.RunningState.Set(false)
		}()
		fyne.Do(func() { d.Hide() })
	}
	closeButton.OnTapped = func() { fyne.Do(func() { d.Hide() }) }
	fyne.Do(func() { d.Show() })
}

func ShowAutoHideInfoUtil(ac *AppController, title, message string) {
	ac.Application.SendNotification(&fyne.Notification{Title: title, Content: message})
	fyne.Do(func() {
		d := dialog.NewCustomWithoutButtons(title, widget.NewLabel(message), ac.MainWindow)
		d.Show()
		go func() {
			time.Sleep(2 * time.Second)
			fyne.Do(func() { d.Hide() })
		}()
	})
}

// AutoLoadProxies attempts to load proxies with retry intervals (1, 3, 7, 13, 17 seconds)
func (ac *AppController) AutoLoadProxies() {
	// Check if already in progress
	ac.AutoLoadMutex.Lock()
	if ac.AutoLoadInProgress {
		ac.AutoLoadMutex.Unlock()
		log.Printf("AutoLoadProxies: Already in progress, skipping")
		return
	}
	ac.AutoLoadInProgress = true
	ac.AutoLoadMutex.Unlock()

	if !ac.ClashAPIEnabled {
		ac.AutoLoadMutex.Lock()
		ac.AutoLoadInProgress = false
		ac.AutoLoadMutex.Unlock()
		log.Printf("AutoLoadProxies: Clash API is disabled, skipping")
		return
	}

	ac.APIStateMutex.RLock()
	selectedGroup := ac.SelectedClashGroup
	ac.APIStateMutex.RUnlock()

	if selectedGroup == "" {
		ac.AutoLoadMutex.Lock()
		ac.AutoLoadInProgress = false
		ac.AutoLoadMutex.Unlock()
		log.Printf("AutoLoadProxies: No group selected, skipping")
		return
	}

	// Retry intervals: 1, 3, 7, 13, 17 seconds
	intervals := []time.Duration{1, 3, 7, 13, 17}

	go func() {
		for attempt, interval := range intervals {
			// Wait for the interval (except first attempt)
			if attempt > 0 {
				time.Sleep(interval * time.Second)
			}

			log.Printf("AutoLoadProxies: Attempt %d/%d to load proxies for group '%s'", attempt+1, len(intervals), selectedGroup)

			// Get current group (it might have changed)
			ac.APIStateMutex.RLock()
			currentGroup := ac.SelectedClashGroup
			baseURL := ac.ClashAPIBaseURL
			token := ac.ClashAPIToken
			ac.APIStateMutex.RUnlock()

			if currentGroup == "" {
				log.Printf("AutoLoadProxies: Group cleared, stopping attempts")
				return
			}

			// Try to load proxies
			proxies, now, err := api.GetProxiesInGroup(baseURL, token, currentGroup, ac.ApiLogFile)
			if err != nil {
				log.Printf("AutoLoadProxies: Attempt %d failed: %v", attempt+1, err)
				// Continue to next attempt
				continue
			}

			// Success - update proxies list
			fyne.Do(func() {
				ac.SetProxiesList(proxies)
				ac.SetActiveProxyName(now)

				// Update tray menu
				if ac.UpdateTrayMenuFunc != nil {
					ac.UpdateTrayMenuFunc()
				}

				// Update UI if callbacks are set
				if ac.ProxiesListWidget != nil {
					ac.ProxiesListWidget.Refresh()
				}
				if ac.ListStatusLabel != nil {
					ac.ListStatusLabel.SetText(fmt.Sprintf("Proxies loaded for '%s'. Active: %s", currentGroup, now))
				}
				if ac.RefreshAPIFunc != nil {
					ac.RefreshAPIFunc()
				}
			})

			log.Printf("AutoLoadProxies: Successfully loaded %d proxies for group '%s' on attempt %d", len(proxies), currentGroup, attempt+1)

			ac.AutoLoadMutex.Lock()
			ac.AutoLoadInProgress = false
			ac.AutoLoadMutex.Unlock()
			return // Success, stop retrying
		}

		log.Printf("AutoLoadProxies: All %d attempts failed", len(intervals))
		ac.AutoLoadMutex.Lock()
		ac.AutoLoadInProgress = false
		ac.AutoLoadMutex.Unlock()
	}()
}

// CreateTrayMenu creates the system tray menu with proxy selection submenu
func (ac *AppController) CreateTrayMenu() *fyne.Menu {
	// Get proxies from current group
	ac.APIStateMutex.RLock()
	proxies := ac.ProxiesList
	activeProxy := ac.ActiveProxyName
	selectedGroup := ac.SelectedClashGroup
	clashAPIEnabled := ac.ClashAPIEnabled
	ac.APIStateMutex.RUnlock()

	// Auto-load proxies if list is empty and API is enabled
	if clashAPIEnabled && selectedGroup != "" && len(proxies) == 0 {
		ac.AutoLoadProxies()
	}

	// Create proxy submenu items
	var proxyMenuItems []*fyne.MenuItem
	if clashAPIEnabled && selectedGroup != "" && len(proxies) > 0 {
		for i := range proxies {
			proxy := proxies[i]
			proxyName := proxy.Name
			isActive := proxyName == activeProxy

			// Create local copy for closure
			pName := proxyName
			menuItem := fyne.NewMenuItem(proxyName, func() {
				// Switch to selected proxy
				go func() {
					err := api.SwitchProxy(ac.ClashAPIBaseURL, ac.ClashAPIToken, selectedGroup, pName, ac.ApiLogFile)
					fyne.Do(func() {
						if err != nil {
							log.Printf("CreateTrayMenu: Failed to switch proxy: %v", err)
							ac.ShowErrorDialog(fmt.Errorf("failed to switch proxy: %w", err))
						} else {
							ac.SetActiveProxyName(pName)
							// Update tray menu after switch
							if ac.UpdateTrayMenuFunc != nil {
								ac.UpdateTrayMenuFunc()
							}
							// Refresh UI if callback is set
							if ac.RefreshAPIFunc != nil {
								ac.RefreshAPIFunc()
							}
						}
					})
				}()
			})

			// Mark active proxy with checkmark
			if isActive {
				menuItem.Label = "✓ " + proxyName
			}

			proxyMenuItems = append(proxyMenuItems, menuItem)
		}
	} else {
		// Show disabled item if no proxies available
		disabledItem := fyne.NewMenuItem("No proxies available", nil)
		disabledItem.Disabled = true
		proxyMenuItems = append(proxyMenuItems, disabledItem)
	}

	// Create proxy submenu
	proxySubmenu := fyne.NewMenu("Select Proxy", proxyMenuItems...)

	// Create main menu items
	menuItems := []*fyne.MenuItem{
		fyne.NewMenuItem("Open", func() { ac.MainWindow.Show() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Start VPN", ac.StartSingBox),
		fyne.NewMenuItem("Stop VPN", ac.StopSingBox),
		fyne.NewMenuItemSeparator(),
	}

	// Add proxy submenu if Clash API is enabled
	if clashAPIEnabled && selectedGroup != "" {
		selectProxyItem := fyne.NewMenuItem("Select Proxy", nil)
		selectProxyItem.ChildMenu = proxySubmenu
		menuItems = append(menuItems, selectProxyItem)
		menuItems = append(menuItems, fyne.NewMenuItemSeparator())
	}

	// Add Quit item
	menuItems = append(menuItems, fyne.NewMenuItem("Quit", ac.GracefulExit))

	return fyne.NewMenu("Singbox Launcher", menuItems...)
}
