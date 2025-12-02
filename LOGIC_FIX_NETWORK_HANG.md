# Детальная логика исправления зависания при отсутствии интернета

## Общая стратегия

Все сетевые операции должны:
1. Выполняться в отдельных горутинах (не в UI потоке)
2. Иметь таймауты на подключение (Dial) и на запрос
3. Использовать `context.Context` для отмены
4. Корректно обрабатывать ошибки сети

## Пошаговая логика исправления

### Шаг 1: Создание утилит для сетевых операций

**Файл:** `core/network_utils.go` (новый)

**Цель:** Централизованные утилиты для всех сетевых операций

**Логика:**
```go
package core

import (
    "context"
    "net"
    "net/http"
    "time"
)

const (
    // Таймауты для сетевых операций
    NetworkDialTimeout    = 5 * time.Second   // Таймаут на подключение
    NetworkRequestTimeout = 15 * time.Second  // Таймаут на запрос
    NetworkLongTimeout    = 30 * time.Second  // Для длительных операций
)

// Создать HTTP клиент с правильными таймаутами
func createHTTPClient(timeout time.Duration) *http.Client {
    return &http.Client{
        Timeout: timeout,
        Transport: &http.Transport{
            DialContext: (&net.Dialer{
                Timeout:   NetworkDialTimeout,
                KeepAlive: 30 * time.Second,
            }).DialContext,
            MaxIdleConns:          100,
            IdleConnTimeout:       90 * time.Second,
            TLSHandshakeTimeout:   10 * time.Second,
            ExpectContinueTimeout: 1 * time.Second,
            // Отключаем keep-alive для предотвращения зависаний
            DisableKeepAlives: false,
        },
    }
}

// Проверка типа сетевой ошибки
func isNetworkError(err error) bool {
    if err == nil {
        return false
    }
    
    // Проверка на timeout
    if netErr, ok := err.(net.Error); ok {
        if netErr.Timeout() {
            return true
        }
        if netErr.Temporary() {
            return true
        }
    }
    
    // Проверка на отсутствие соединения
    if opErr, ok := err.(*net.OpError); ok {
        return opErr != nil
    }
    
    // Проверка на DNS ошибку
    if _, ok := err.(*net.DNSError); ok {
        return true
    }
    
    // Проверка на контекст (отмена/таймаут)
    if err == context.DeadlineExceeded || err == context.Canceled {
        return true
    }
    
    return false
}

// Получить понятное сообщение об ошибке сети
func getNetworkErrorMessage(err error) string {
    if err == nil {
        return "Unknown network error"
    }
    
    if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
        return "Network timeout: connection timed out"
    }
    
    if opErr, ok := err.(*net.OpError); ok {
        if opErr.Op == "dial" {
            return "Network error: cannot connect to server"
        }
        return "Network error: " + opErr.Error()
    }
    
    if _, ok := err.(*net.DNSError); ok {
        return "DNS error: cannot resolve hostname"
    }
    
    if err == context.DeadlineExceeded {
        return "Request timeout: operation took too long"
    }
    
    return "Network error: " + err.Error()
}
```

### Шаг 2: Исправление core_version.go

**Проблема:** `GetLatestCoreVersion()` может блокировать, если вызывается синхронно

**Логика исправления:**

1. Добавить контекст с таймаутом
2. Использовать универсальный HTTP клиент
3. Улучшить обработку ошибок

**Изменения:**
```go
// getLatestVersionFromURL - добавить контекст
func (ac *AppController) getLatestVersionFromURL(url string) (string, error) {
    // Создаем контекст с таймаутом
    ctx, cancel := context.WithTimeout(context.Background(), NetworkRequestTimeout)
    defer cancel()
    
    // Используем универсальный HTTP клиент
    client := createHTTPClient(NetworkRequestTimeout)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", fmt.Errorf("failed to create request: %w", err)
    }
    
    req.Header.Set("Accept", "application/vnd.github.v3+json")
    req.Header.Set("User-Agent", "singbox-launcher/1.0")
    
    resp, err := client.Do(req)
    if err != nil {
        // Проверяем тип ошибки
        if isNetworkError(err) {
            return "", fmt.Errorf("network error: %s", getNetworkErrorMessage(err))
        }
        return "", fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("HTTP %d", resp.StatusCode)
    }
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read response: %w", err)
    }
    
    var release struct {
        TagName string `json:"tag_name"`
    }
    
    if err := json.Unmarshal(body, &release); err != nil {
        return "", fmt.Errorf("failed to parse response: %w", err)
    }
    
    version := strings.TrimPrefix(release.TagName, "v")
    return version, nil
}
```

### Шаг 3: Исправление subscription_parser.go

**Проблема:** Нет контекста для отмены запроса

**Логика исправления:**

1. Добавить контекст с таймаутом
2. Использовать универсальный HTTP клиент
3. Улучшить обработку ошибок

**Изменения:**
```go
// FetchSubscription - добавить контекст
func FetchSubscription(url string) ([]byte, error) {
    // Создаем контекст с таймаутом
    ctx, cancel := context.WithTimeout(context.Background(), NetworkRequestTimeout)
    defer cancel()
    
    // Используем универсальный HTTP клиент
    client := createHTTPClient(NetworkRequestTimeout)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    req.Header.Set("User-Agent", "singbox-launcher/1.0")
    
    resp, err := client.Do(req)
    if err != nil {
        // Проверяем тип ошибки
        if isNetworkError(err) {
            return nil, fmt.Errorf("network error: %s", getNetworkErrorMessage(err))
        }
        return nil, fmt.Errorf("failed to fetch subscription: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("subscription server returned status %d", resp.StatusCode)
    }
    
    content, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read subscription content: %w", err)
    }
    
    if len(content) == 0 {
        return nil, fmt.Errorf("subscription returned empty content")
    }
    
    decoded, err := DecodeSubscriptionContent(content)
    if err != nil {
        return nil, fmt.Errorf("failed to decode subscription content: %w", err)
    }
    
    return decoded, nil
}
```

### Шаг 4: Исправление core_downloader.go

**Проблема:** Таймаут на подключение может быть недостаточным

**Логика исправления:**

1. Использовать универсальный HTTP клиент
2. Добавить контекст для длительных операций
3. Улучшить обработку ошибок

**Изменения:**
```go
// getReleaseInfoFromGitHub - использовать универсальный клиент
func (ac *AppController) getReleaseInfoFromGitHub(version string) (*ReleaseInfo, error) {
    url := fmt.Sprintf("https://api.github.com/repos/SagerNet/sing-box/releases/tags/v%s", version)
    if version == "" {
        url = "https://api.github.com/repos/SagerNet/sing-box/releases/latest"
    }
    
    // Создаем контекст с таймаутом
    ctx, cancel := context.WithTimeout(context.Background(), NetworkRequestTimeout)
    defer cancel()
    
    // Используем универсальный HTTP клиент
    client := createHTTPClient(NetworkRequestTimeout)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    req.Header.Set("Accept", "application/vnd.github.v3+json")
    req.Header.Set("User-Agent", "singbox-launcher/1.0")
    
    resp, err := client.Do(req)
    if err != nil {
        if isNetworkError(err) {
            return nil, fmt.Errorf("network error: %s", getNetworkErrorMessage(err))
        }
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
    }
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }
    
    var release ReleaseInfo
    if err := json.Unmarshal(body, &release); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }
    
    return &release, nil
}

// downloadFileFromURL - для длительных операций использовать больший таймаут
func (ac *AppController) downloadFileFromURL(url, destPath string, progressChan chan DownloadProgress) error {
    // Для скачивания используем больший таймаут
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    
    // Используем клиент с большим таймаутом для скачивания
    client := createHTTPClient(5 * time.Minute)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }
    
    req.Header.Set("User-Agent", "singbox-launcher/1.0")
    
    resp, err := client.Do(req)
    if err != nil {
        if isNetworkError(err) {
            return fmt.Errorf("network error: %s", getNetworkErrorMessage(err))
        }
        return fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("HTTP %d", resp.StatusCode)
    }
    
    // ... остальной код скачивания ...
}
```

### Шаг 5: Исправление ui/core_dashboard_tab.go

**Проблема:** `updateVersionInfo()` вызывается синхронно в UI потоке

**Логика исправления:**

1. Создать асинхронную версию `updateVersionInfoAsync()`
2. Использовать каналы для синхронизации
3. Обновлять UI только через `fyne.Do()`

**Изменения:**
```go
// updateVersionInfoAsync - асинхронная версия
func (tab *CoreDashboardTab) updateVersionInfoAsync() {
    // Запускаем в горутине
    go func() {
        // Получаем установленную версию (локальная операция, быстрая)
        installedVersion, err := tab.controller.GetInstalledCoreVersion()
        
        // Обновляем UI для установленной версии
        fyne.Do(func() {
            if err != nil {
                tab.singboxStatusLabel.SetText("❌ sing-box.exe not found")
                tab.singboxStatusLabel.Importance = widget.MediumImportance
                tab.downloadButton.SetText("Download")
                tab.downloadButton.Enable()
                tab.downloadButton.Show()
                return
            }
            
            tab.singboxStatusLabel.SetText(installedVersion)
            tab.singboxStatusLabel.Importance = widget.MediumImportance
        })
        
        // Получаем последнюю версию (сетевая операция, асинхронная)
        latest, latestErr := tab.controller.GetLatestCoreVersion()
        
        // Обновляем UI с результатом
        fyne.Do(func() {
            if latestErr != nil {
                // Ошибка сети - не критично, просто не показываем обновление
                log.Printf("updateVersionInfoAsync: failed to get latest version: %v", latestErr)
                tab.downloadButton.Hide()
                return
            }
            
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

// Обновить startAutoUpdate для использования асинхронной версии
func (tab *CoreDashboardTab) startAutoUpdate() {
    go func() {
        rand.Seed(time.Now().UnixNano())
        
        for {
            select {
            case <-tab.stopAutoUpdate:
                return
            default:
                var delay time.Duration
                if tab.lastUpdateSuccess {
                    delay = 10 * time.Minute
                } else {
                    delay = time.Duration(20+rand.Intn(16)) * time.Second
                }
                
                select {
                case <-time.After(delay):
                    // Вызываем асинхронную версию
                    tab.updateVersionInfoAsync()
                    // Устанавливаем успех после небольшой задержки
                    // (в реальности нужно отслеживать через канал)
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
```

### Шаг 6: Улучшение api/clash.go

**Проблема:** Таймауты есть, но можно улучшить обработку ошибок

**Логика исправления:**

1. Использовать универсальные утилиты для проверки ошибок
2. Улучшить сообщения об ошибках

**Изменения:**
```go
// В функциях TestAPIConnection, GetProxiesInGroup, SwitchProxy, GetDelay
// Добавить проверку сетевых ошибок:

resp, err := httpClient.Do(req)
if err != nil {
    if isNetworkError(err) {
        return fmt.Errorf("network error: %s", getNetworkErrorMessage(err))
    }
    return fmt.Errorf("failed to execute request: %w", err)
}
```

## Порядок внедрения

1. **Создать `core/network_utils.go`** - утилиты для сетевых операций
2. **Исправить `core/core_version.go`** - добавить контекст и обработку ошибок
3. **Исправить `core/subscription_parser.go`** - добавить контекст
4. **Исправить `core/core_downloader.go`** - улучшить таймауты
5. **Исправить `ui/core_dashboard_tab.go`** - сделать асинхронным
6. **Улучшить `api/clash.go`** - обработка ошибок

## Тестирование

После каждого шага проверять:
1. Приложение не зависает при отсутствии интернета
2. UI остается отзывчивым
3. Ошибки обрабатываются корректно
4. Таймауты работают (запрос завершается не более чем за 30 секунд)

## Критерии успеха

✅ Все сетевые операции имеют таймауты
✅ Все сетевые операции выполняются в горутинах
✅ UI никогда не блокируется сетевыми запросами
✅ Пользователь видит понятные сообщения об ошибках
✅ Логи содержат детальную информацию об ошибках


