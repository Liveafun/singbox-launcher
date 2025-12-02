package ui

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/core"
)

// CoreDashboardTab управляет вкладкой Core Dashboard
type CoreDashboardTab struct {
	controller *core.AppController

	// UI элементы
	statusLabel             *widget.Label // Полный статус: "Core Status" + иконка + текст
	singboxStatusLabel      *widget.Label // Статус sing-box (версия или "not found")
	downloadButton          *widget.Button
	downloadProgress        *widget.ProgressBar // Прогресс-бар для скачивания
	downloadContainer       fyne.CanvasObject   // Контейнер для кнопки/прогресс-бара
	startButton             *widget.Button      // Кнопка Start
	stopButton              *widget.Button      // Кнопка Stop
	wintunStatusLabel       *widget.Label       // Статус wintun.dll
	wintunDownloadButton    *widget.Button      // Кнопка скачивания wintun.dll
	wintunDownloadProgress  *widget.ProgressBar // Прогресс-бар для скачивания wintun.dll
	wintunDownloadContainer fyne.CanvasObject   // Контейнер для кнопки/прогресс-бара wintun

	// Данные
	stopAutoUpdate           chan bool
	lastUpdateSuccess        bool // Отслеживаем успех последнего обновления версии
	downloadInProgress       bool // Флаг процесса скачивания sing-box
	wintunDownloadInProgress bool // Флаг процесса скачивания wintun.dll
}

// CreateCoreDashboardTab создает и возвращает вкладку Core Dashboard
func CreateCoreDashboardTab(ac *core.AppController) fyne.CanvasObject {
	tab := &CoreDashboardTab{
		controller:     ac,
		stopAutoUpdate: make(chan bool),
	}

	// Блок статуса с кнопками в одну строку
	statusRow := tab.createStatusRow()

	// Блок версии и пути
	versionBlock := tab.createVersionBlock()

	// Блок wintun.dll (только для Windows)
	var wintunBlock fyne.CanvasObject
	if runtime.GOOS == "windows" {
		wintunBlock = tab.createWintunBlock()
	}

	// Основной контейнер - все элементы в VBox, кнопка Exit в конце
	contentItems := []fyne.CanvasObject{
		statusRow,
		widget.NewSeparator(),
		versionBlock,
	}
	if runtime.GOOS == "windows" && wintunBlock != nil {
		contentItems = append(contentItems, wintunBlock) // Убрали separator перед wintunBlock
	}

	// Горизонтальная линия и кнопка Exit в конце списка
	contentItems = append(contentItems, widget.NewSeparator())
	exitButton := widget.NewButton("Exit", ac.GracefulExit)
	contentItems = append(contentItems, exitButton)

	content := container.NewVBox(contentItems...)

	// Регистрируем callback для обновления статуса при изменении RunningState
	tab.controller.UpdateCoreStatusFunc = func() {
		fyne.Do(func() {
			tab.updateRunningStatus()
		})
	}

	// Первоначальное обновление
	tab.updateBinaryStatus() // Проверяет наличие бинарника и вызывает updateRunningStatus
	tab.updateVersionInfo()
	if runtime.GOOS == "windows" {
		tab.updateWintunStatus() // Проверяет наличие wintun.dll
	}

	// Запускаем автообновление версии
	tab.startAutoUpdate()

	return content
}

// createStatusRow создает строку со статусом и кнопками
func (tab *CoreDashboardTab) createStatusRow() fyne.CanvasObject {
	// Объединяем все в один label: "Core Status" + иконка + текст статуса
	tab.statusLabel = widget.NewLabel("Core Status Checking...")
	tab.statusLabel.Wrapping = fyne.TextWrapOff       // Отключаем перенос текста
	tab.statusLabel.Alignment = fyne.TextAlignLeading // Выравнивание текста
	tab.statusLabel.Importance = widget.MediumImportance

	startButton := widget.NewButton("Start", func() {
		tab.controller.StartSingBox()
		// Статус обновится автоматически через UpdateCoreStatusFunc
	})

	stopButton := widget.NewButton("Stop", func() {
		tab.controller.StopSingBox()
		// Статус обновится автоматически через UpdateCoreStatusFunc
	})

	// Сохраняем ссылки на кнопки для обновления блокировок
	tab.startButton = startButton
	tab.stopButton = stopButton

	// Статус в одну строку - все в одном label
	statusContainer := container.NewHBox(
		tab.statusLabel, // "Core Status" + иконка + текст статуса
	)

	// Кнопки на новой строке по центру
	buttonsContainer := container.NewCenter(
		container.NewHBox(startButton, stopButton),
	)

	// Возвращаем контейнер со статусом и кнопками, с пустыми строками до и после кнопок
	return container.NewVBox(
		statusContainer,
		widget.NewLabel(""), // Пустая строка перед кнопками
		buttonsContainer,
		widget.NewLabel(""), // Пустая строка после кнопок
	)
}

// createVersionBlock создает блок с версией (по аналогии с wintun)
func (tab *CoreDashboardTab) createVersionBlock() fyne.CanvasObject {
	versionTitle := widget.NewLabel("Sing-box Ver.")
	versionTitle.Importance = widget.MediumImportance

	// Статус sing-box (версия или "not found") - по аналогии с wintunStatusLabel
	tab.singboxStatusLabel = widget.NewLabel("Checking...")
	tab.singboxStatusLabel.Wrapping = fyne.TextWrapOff

	// Кнопка Download/Update справа от статуса
	tab.downloadButton = widget.NewButton("Download", func() {
		tab.handleDownload()
	})
	tab.downloadButton.Importance = widget.MediumImportance
	tab.downloadButton.Disable() // По умолчанию отключена, пока не проверим наличие бинарника

	// Прогресс-бар для скачивания (скрыт по умолчанию)
	tab.downloadProgress = widget.NewProgressBar()
	tab.downloadProgress.Hide()
	tab.downloadProgress.SetValue(0)

	// Контейнер для кнопки/прогресс-бара - они занимают одно место, переключаются через Show/Hide
	// Структура точно такая же, как у wintun
	progressContainer := container.NewMax(tab.downloadProgress)
	tab.downloadContainer = container.NewStack(tab.downloadButton, progressContainer)

	// Объединяем статус и кнопку в одну строку с фиксированной шириной для правой части
	singboxInfoContainer := container.NewGridWithColumns(2,
		tab.singboxStatusLabel,
		tab.downloadContainer,
	)

	return container.NewVBox(
		container.NewHBox(versionTitle, singboxInfoContainer),
	)
}

// updateBinaryStatus проверяет наличие бинарника и обновляет статус
func (tab *CoreDashboardTab) updateBinaryStatus() {
	// Проверяем, существует ли бинарник
	if _, err := tab.controller.GetInstalledCoreVersion(); err != nil {
		tab.statusLabel.SetText("Core Status ❌ Error: sing-box not found")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
		// Обновляем иконку трея (красная при ошибке)
		tab.controller.UpdateUI()
		return
	}
	// Если бинарник найден, обновляем статус запуска
	tab.updateRunningStatus()
	// Обновляем иконку трея (может измениться с красной на черную/зеленую)
	tab.controller.UpdateUI()
}

// updateRunningStatus обновляет статус Running/Stopped на основе RunningState
func (tab *CoreDashboardTab) updateRunningStatus() {
	// Проверяем, существует ли бинарник (если нет - показываем ошибку)
	if _, err := tab.controller.GetInstalledCoreVersion(); err != nil {
		tab.statusLabel.SetText("Core Status ❌ Error: sing-box not found")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
		// Блокируем кнопки если бинарника нет
		if tab.startButton != nil {
			tab.startButton.Disable()
		}
		if tab.stopButton != nil {
			tab.stopButton.Disable()
		}
		return
	}

	// Обновляем статус на основе RunningState
	if tab.controller.RunningState.IsRunning() {
		tab.statusLabel.SetText("Core Status ✅ Running")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
		// Блокируем Start, разблокируем Stop
		if tab.startButton != nil {
			tab.startButton.Disable()
		}
		if tab.stopButton != nil {
			tab.stopButton.Enable()
		}
	} else {
		tab.statusLabel.SetText("Core Status ⏸️ Stopped")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
		// Блокируем Stop, разблокируем Start
		if tab.startButton != nil {
			tab.startButton.Enable()
		}
		if tab.stopButton != nil {
			tab.stopButton.Disable()
		}
	}
}

// updateVersionInfo обновляет информацию о версии (по аналогии с updateWintunStatus)
// Теперь полностью асинхронная - не блокирует UI
func (tab *CoreDashboardTab) updateVersionInfo() error {
	// Запускаем асинхронное обновление
	tab.updateVersionInfoAsync()
	return nil
}

// updateVersionInfoAsync - асинхронная версия обновления информации о версии
func (tab *CoreDashboardTab) updateVersionInfoAsync() {
	// Запускаем в горутине
	go func() {
		// Получаем установленную версию (локальная операция, быстрая)
		installedVersion, err := tab.controller.GetInstalledCoreVersion()

		// Обновляем UI для установленной версии
		fyne.Do(func() {
			if err != nil {
				// Показываем ошибку в статусе
				tab.singboxStatusLabel.SetText("❌ sing-box.exe not found")
				tab.singboxStatusLabel.Importance = widget.MediumImportance
				tab.downloadButton.SetText("Download")
				tab.downloadButton.Enable()
				tab.downloadButton.Importance = widget.HighImportance
				tab.downloadButton.Show()
			} else {
				// Показываем версию
				tab.singboxStatusLabel.SetText(installedVersion)
				tab.singboxStatusLabel.Importance = widget.MediumImportance
			}
		})

		// Если бинарник не найден, пытаемся получить последнюю версию для кнопки
		if err != nil {
			latest, latestErr := tab.controller.GetLatestCoreVersion()
			fyne.Do(func() {
				if latestErr == nil && latest != "" {
					tab.downloadButton.SetText(fmt.Sprintf("Download v%s", latest))
				} else {
					tab.downloadButton.SetText("Download")
				}
			})
			return
		}

		// Получаем последнюю версию (сетевая операция, асинхронная)
		latest, latestErr := tab.controller.GetLatestCoreVersion()

		// Обновляем UI с результатом
		fyne.Do(func() {
			if latestErr != nil {
				// Ошибка сети - не критично, просто не показываем обновление
				// Логируем для отладки, но не показываем пользователю
				tab.downloadButton.Hide()
				return
			}

			// Сравниваем версии
			if latest != "" && compareVersions(installedVersion, latest) < 0 {
				// Есть обновление
				tab.downloadButton.SetText(fmt.Sprintf("Update v%s", latest))
				tab.downloadButton.Enable()
				tab.downloadButton.Importance = widget.HighImportance
				tab.downloadButton.Show()
			} else {
				// Версия актуальна
				tab.downloadButton.Hide()
			}
		})
	}()
}

// compareVersions сравнивает две версии (формат X.Y.Z)
// Возвращает: -1 если v1 < v2, 0 если v1 == v2, 1 если v1 > v2
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &num1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &num2)
		}

		if num1 < num2 {
			return -1
		}
		if num1 > num2 {
			return 1
		}
	}

	return 0
}

// handleDownload обрабатывает нажатие на кнопку Download
func (tab *CoreDashboardTab) handleDownload() {
	if tab.downloadInProgress {
		return // Уже идет скачивание
	}

	// Получаем информацию о версиях (локальная операция)
	versionInfo := tab.controller.GetCoreVersionInfo()

	targetVersion := versionInfo.LatestVersion
	if targetVersion == "" {
		// Пытаемся получить последнюю версию асинхронно
		// Но для скачивания нужна версия сразу, поэтому делаем синхронно в горутине
		go func() {
			latest, err := tab.controller.GetLatestCoreVersion()
			fyne.Do(func() {
				if err != nil {
					ShowError(tab.controller.MainWindow, fmt.Errorf("failed to get latest version: %w", err))
					tab.downloadInProgress = false
					tab.downloadButton.Enable()
					tab.downloadButton.Show()
					return
				}
				// Запускаем скачивание с полученной версией
				tab.startDownloadWithVersion(latest)
			})
		}()
		return
	}

	// Запускаем скачивание с известной версией
	tab.startDownloadWithVersion(targetVersion)
}

// startDownloadWithVersion запускает процесс скачивания с указанной версией
func (tab *CoreDashboardTab) startDownloadWithVersion(targetVersion string) {
	// Запускаем скачивание в отдельной горутине
	tab.downloadInProgress = true
	tab.downloadButton.Disable()
	// Скрываем кнопку и показываем прогресс-бар
	tab.downloadButton.Hide()
	tab.downloadProgress.Show()
	tab.downloadProgress.SetValue(0)

	// Создаем канал для прогресса
	progressChan := make(chan core.DownloadProgress, 10)

	// Запускаем скачивание в отдельной горутине
	go func() {
		tab.controller.DownloadCore(targetVersion, progressChan)
	}()

	// Обрабатываем прогресс в отдельной горутине
	go func() {
		for progress := range progressChan {
			fyne.Do(func() {
				// Обновляем только прогресс-бар (кнопка скрыта)
				tab.downloadProgress.SetValue(float64(progress.Progress) / 100.0)

				if progress.Status == "done" {
					tab.downloadInProgress = false
					// Скрываем прогресс-бар и показываем кнопку
					tab.downloadProgress.Hide()
					tab.downloadProgress.SetValue(0)
					tab.downloadButton.Show()
					tab.downloadButton.Enable()
					// Обновляем статусы после успешного скачивания (это уберет ошибки и обновит статус)
					tab.updateVersionInfo()
					tab.updateBinaryStatus() // Это вызовет updateRunningStatus() и обновит статус
					// Обновляем иконку трея (может измениться с красной на черную/зеленую)
					tab.controller.UpdateUI()
					ShowInfo(tab.controller.MainWindow, "Download Complete", progress.Message)
				} else if progress.Status == "error" {
					tab.downloadInProgress = false
					// Скрываем прогресс-бар и показываем кнопку
					tab.downloadProgress.Hide()
					tab.downloadProgress.SetValue(0)
					tab.downloadButton.Show()
					tab.downloadButton.Enable()
					ShowError(tab.controller.MainWindow, progress.Error)
				}
			})
		}
	}()
}

// startAutoUpdate запускает автообновление версии (статус управляется через RunningState)
func (tab *CoreDashboardTab) startAutoUpdate() {
	// Запускаем периодическое обновление с умной логикой
	go func() {
		rand.Seed(time.Now().UnixNano()) // Инициализация генератора случайных чисел

		for {
			select {
			case <-tab.stopAutoUpdate:
				return
			default:
				// Ждем перед следующим обновлением
				var delay time.Duration
				if tab.lastUpdateSuccess {
					// Если последнее обновление было успешным - не повторяем автоматически
					// Ждем очень долго (или можно вообще не повторять)
					delay = 10 * time.Minute
				} else {
					// Если была ошибка - повторяем через случайный интервал 20-35 секунд
					delay = time.Duration(20+rand.Intn(16)) * time.Second // 20-35 секунд
				}

				select {
				case <-time.After(delay):
					// Обновляем только версию асинхронно (не блокируем UI)
					// updateVersionInfo теперь полностью асинхронная
					tab.updateVersionInfo()
					// Устанавливаем успех после небольшой задержки
					// (в реальности нужно отслеживать через канал, но для простоты используем задержку)
					go func() {
						time.Sleep(2 * time.Second)
						tab.lastUpdateSuccess = true // Упрощенная логика
					}()
				case <-tab.stopAutoUpdate:
					return
				}
			}
		}
	}()
}

// createWintunBlock создает блок для отображения статуса wintun.dll
func (tab *CoreDashboardTab) createWintunBlock() fyne.CanvasObject {
	wintunTitle := widget.NewLabel("WinTun DLL")
	wintunTitle.Importance = widget.MediumImportance

	tab.wintunStatusLabel = widget.NewLabel("Checking...")
	tab.wintunStatusLabel.Wrapping = fyne.TextWrapOff

	// Кнопка скачивания wintun.dll
	tab.wintunDownloadButton = widget.NewButton("Download", func() {
		tab.handleWintunDownload()
	})
	tab.wintunDownloadButton.Importance = widget.MediumImportance
	tab.wintunDownloadButton.Disable() // По умолчанию отключена

	// Прогресс-бар для скачивания wintun.dll
	tab.wintunDownloadProgress = widget.NewProgressBar()
	tab.wintunDownloadProgress.Hide()
	tab.wintunDownloadProgress.SetValue(0)

	// Контейнер для кнопки/прогресс-бара wintun
	progressContainer := container.NewMax(tab.wintunDownloadProgress)
	tab.wintunDownloadContainer = container.NewStack(tab.wintunDownloadButton, progressContainer)

	// Объединяем статус и кнопку в одну строку с фиксированной шириной для правой части
	wintunInfoContainer := container.NewGridWithColumns(2,
		tab.wintunStatusLabel,
		tab.wintunDownloadContainer,
	)

	return container.NewVBox(
		container.NewHBox(wintunTitle, wintunInfoContainer),
	)
}

// updateWintunStatus обновляет статус wintun.dll
func (tab *CoreDashboardTab) updateWintunStatus() {
	if runtime.GOOS != "windows" {
		return // wintun нужен только на Windows
	}

	exists, err := tab.controller.CheckWintunDLL()
	if err != nil {
		tab.wintunStatusLabel.SetText("❌ Error checking wintun.dll")
		tab.wintunStatusLabel.Importance = widget.MediumImportance
		tab.wintunDownloadButton.Disable()
		return
	}

	if exists {
		tab.wintunStatusLabel.SetText("ok")
		tab.wintunStatusLabel.Importance = widget.MediumImportance
		tab.wintunDownloadButton.Hide()
		tab.wintunDownloadProgress.Hide()
	} else {
		tab.wintunStatusLabel.SetText("❌ wintun.dll not found")
		tab.wintunStatusLabel.Importance = widget.MediumImportance
		tab.wintunDownloadButton.Show()
		tab.wintunDownloadButton.Enable()
		tab.wintunDownloadButton.SetText("Download wintun.dll")
		tab.wintunDownloadButton.Importance = widget.HighImportance
	}
}

// handleWintunDownload обрабатывает нажатие на кнопку Download wintun.dll
func (tab *CoreDashboardTab) handleWintunDownload() {
	if tab.wintunDownloadInProgress {
		return // Уже идет скачивание
	}

	tab.wintunDownloadInProgress = true
	tab.wintunDownloadButton.Disable()
	tab.wintunDownloadButton.SetText("Downloading...")
	tab.wintunDownloadProgress.Show()
	tab.wintunDownloadProgress.SetValue(0)

	go func() {
		progressChan := make(chan core.DownloadProgress, 10)

		go func() {
			tab.controller.DownloadWintunDLL(progressChan)
		}()

		for progress := range progressChan {
			fyne.Do(func() {
				tab.wintunDownloadProgress.SetValue(float64(progress.Progress) / 100.0)
				tab.wintunDownloadButton.SetText(fmt.Sprintf("Downloading... %d%%", progress.Progress))

				if progress.Status == "done" {
					tab.wintunDownloadInProgress = false
					tab.updateWintunStatus() // Обновляем статус после скачивания
					tab.wintunDownloadProgress.Hide()
					tab.wintunDownloadProgress.SetValue(0)
					tab.wintunDownloadButton.Enable()
					ShowInfo(tab.controller.MainWindow, "Download Complete", progress.Message)
				} else if progress.Status == "error" {
					tab.wintunDownloadInProgress = false
					tab.wintunDownloadProgress.Hide()
					tab.wintunDownloadProgress.SetValue(0)
					tab.wintunDownloadButton.Show()
					tab.wintunDownloadButton.Enable()
					ShowError(tab.controller.MainWindow, progress.Error)
				}
			})
		}
	}()
}
