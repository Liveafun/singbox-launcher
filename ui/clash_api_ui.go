package ui

import (
	"fmt"
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/api"
	"singbox-launcher/core"
)

// CreateClashAPIContent creates and returns the content for the "Clash API" tab.
func CreateClashAPIContent(ac *core.AppController) fyne.CanvasObject {
	ac.ApiStatusLabel = widget.NewLabel("Status: Not checked")
	status := widget.NewLabel("Click 'Load Proxies' or 'Test API'")
	ac.ListStatusLabel = status

	selectorOptions, defaultSelector, err := core.GetSelectorGroupsFromConfig(ac.ConfigPath)
	if err != nil {
		log.Printf("clash_api_ui: failed to get selector groups: %v", err)
	}
	if len(selectorOptions) == 0 {
		selectorOptions = []string{"proxy-out"}
	}
	selectedGroup := defaultSelector
	if selectedGroup == "" {
		selectedGroup = selectorOptions[0]
	}
	ac.SelectedClashGroup = selectedGroup

	var (
		groupSelect            *widget.Select
		suppressSelectCallback bool
	)

	// --- Логика обновления и сброса ---

	onLoadAndRefreshProxies := func() {
		if !ac.ClashAPIEnabled {
			ac.ShowAutoHideInfo("Clash API", "API is disabled: config error")
			if ac.ListStatusLabel != nil {
				ac.ListStatusLabel.SetText("Clash API disabled due to config error")
			}
			return
		}
		group := selectedGroup
		if group == "" {
			return
		}
		if ac.ListStatusLabel != nil {
			ac.ListStatusLabel.SetText(fmt.Sprintf("Loading proxies for '%s'...", group))
		}
		go func(group string) {
			proxies, now, err := api.GetProxiesInGroup(ac.ClashAPIBaseURL, ac.ClashAPIToken, group, ac.ApiLogFile)
			fyne.Do(func() {
				if err != nil {
					ac.ShowAutoHideInfo("Clash API Error", "Failed to load proxies: "+err.Error())
					if ac.ListStatusLabel != nil {
						ac.ListStatusLabel.SetText("Error: " + err.Error())
					}
					return
				}

				ac.SetProxiesList(proxies)
				ac.SetActiveProxyName(now)

				if ac.ProxiesListWidget != nil {
					ac.ProxiesListWidget.Refresh()
				}

				if ac.ListStatusLabel != nil {
					ac.ListStatusLabel.SetText(fmt.Sprintf("Proxies loaded for '%s'. Active: %s", group, now))
				}
			})
		}(group)
	}

	onTestAPIConnection := func() {
		if !ac.ClashAPIEnabled {
			ac.ApiStatusLabel.SetText("❌ API Off (Config Error)")
			ac.ShowAutoHideInfo("Clash API", "API is disabled: config error")
			return
		}
		go func() {
			err := api.TestAPIConnection(ac.ClashAPIBaseURL, ac.ClashAPIToken, ac.ApiLogFile)
			fyne.Do(func() {
				if err != nil {
					ac.ApiStatusLabel.SetText("❌ API Off (Error)")
					ac.ShowAutoHideInfo("Clash API Error", "API connection failed: "+err.Error())
					return
				}
				ac.ApiStatusLabel.SetText("✅ API On")
				onLoadAndRefreshProxies()
			})
		}()
	}

	onResetAPIState := func() {
		log.Println("clash_api_ui: Resetting API state.")
		ac.SetProxiesList([]api.ProxyInfo{})
		ac.SetActiveProxyName("")
		ac.SetSelectedIndex(-1)
		if ac.ApiStatusLabel != nil {
			ac.ApiStatusLabel.SetText("Status: Not running")
		}
		if ac.ListStatusLabel != nil {
			ac.ListStatusLabel.SetText("Sing-box is stopped.")
		}
		if ac.ProxiesListWidget != nil {
			ac.ProxiesListWidget.Refresh()
		}
	}

	// --- Регистрация колбэков в контроллере ---
	ac.RefreshAPIFunc = onTestAPIConnection
	ac.ResetAPIStateFunc = onResetAPIState

	// --- Вспомогательная функция для пинга ---
	pingProxy := func(proxyName string, button *widget.Button) {
		go func() {
			fyne.Do(func() { button.SetText("...") })
			delay, err := api.GetDelay(ac.ClashAPIBaseURL, ac.ClashAPIToken, proxyName, ac.ApiLogFile)
			fyne.Do(func() {
				if err != nil {
					button.SetText("Error")
					status.SetText("Delay error: " + err.Error())
				} else {
					button.SetText(fmt.Sprintf("%d ms", delay))
					status.SetText(fmt.Sprintf("Delay: %d ms for %s", delay, proxyName))
				}
			})
		}()
	}

	// --- Создание виджета списка ---

	createItem := func() fyne.CanvasObject {
		background := canvas.NewRectangle(color.Transparent)
		background.CornerRadius = 5

		nameLabel := widget.NewLabel("Proxy Name")
		nameLabel.TextStyle.Bold = true

		pingButton := widget.NewButton("Ping", nil)
		switchButton := widget.NewButton("▶️", nil)

		// ИЗМЕНЕНО: Убрали VBox и метку трафика.
		content := container.NewHBox(
			nameLabel, // Сразу метка с именем
			layout.NewSpacer(),
			pingButton,
			switchButton,
		)

		paddedContent := container.NewPadded(content)
		return container.NewStack(background, paddedContent)
	}

	updateItem := func(id int, o fyne.CanvasObject) {
		proxies := ac.GetProxiesList()
		if id < 0 || id >= len(proxies) {
			return
		}
		proxyInfo := proxies[id]

		stack := o.(*fyne.Container)
		background := stack.Objects[0].(*canvas.Rectangle)
		paddedContent := stack.Objects[1].(*fyne.Container)
		content := paddedContent.Objects[0].(*fyne.Container)

		nameLabel := content.Objects[0].(*widget.Label)
		pingButton := content.Objects[2].(*widget.Button)
		switchButton := content.Objects[3].(*widget.Button)

		nameLabel.SetText(proxyInfo.Name)

		// ИЗМЕНЕНО: Устанавливаем начальное значение пинга, если оно есть.
		if proxyInfo.Delay > 0 {
			pingButton.SetText(fmt.Sprintf("%d ms", proxyInfo.Delay))
		} else {
			pingButton.SetText("Ping")
		}

		// Обновляем фон
		if proxyInfo.Name == ac.GetActiveProxyName() {
			background.FillColor = color.NRGBA{R: 144, G: 238, B: 144, A: 128}
		} else if id == ac.GetSelectedIndex() {
			background.FillColor = color.Gray{0xDD}
		} else {
			background.FillColor = color.Transparent
		}
		background.Refresh()

		// Обновляем колбэки кнопок
		proxyNameForCallback := proxyInfo.Name

		pingButton.OnTapped = func() {
			pingProxy(proxyNameForCallback, pingButton)
		}

		// ИЗМЕНЕНО: Новая логика переключения.
		switchButton.OnTapped = func() {
			if !ac.ClashAPIEnabled {
				ac.ShowAutoHideInfo("Clash API", "API is disabled: config error")
				return
			}
			go func(group string) {
				err := api.SwitchProxy(ac.ClashAPIBaseURL, ac.ClashAPIToken, group, proxyNameForCallback, ac.ApiLogFile)
				fyne.Do(func() {
					if err != nil {
						ac.ShowAutoHideInfo("Clash API Error", "Switch error: "+err.Error())
						status.SetText("Switch error: " + err.Error())
					} else {
						// 1. Обновляем состояние в контроллере
						ac.SetActiveProxyName(proxyNameForCallback)
						// 2. Перерисовываем список для обновления выделения
						ac.ProxiesListWidget.Refresh()
						// 3. Сразу запускаем пинг для этого прокси
						pingProxy(proxyNameForCallback, pingButton)
						if ac.ListStatusLabel != nil {
							ac.ListStatusLabel.SetText(fmt.Sprintf("Switched '%s' to %s", group, proxyNameForCallback))
						}
					}
				})
			}(selectedGroup)
		}
	}

	proxiesListWidget := widget.NewList(
		func() int { return len(ac.GetProxiesList()) },
		createItem,
		updateItem,
	)

	proxiesListWidget.OnSelected = func(id int) {
		ac.SetSelectedIndex(id)
		proxies := ac.GetProxiesList()
		if id >= 0 && id < len(proxies) {
			status.SetText("Selected: " + proxies[id].Name)
		}
		proxiesListWidget.Refresh()
	}

	ac.ProxiesListWidget = proxiesListWidget

	// --- Сборка всего контента ---
	scrollContainer := container.NewScroll(proxiesListWidget)
	scrollContainer.SetMinSize(fyne.NewSize(0, 300))

	loadButton := widget.NewButton("Load Proxies", onLoadAndRefreshProxies)
	testAPIButton := widget.NewButton("Test API Connection", onTestAPIConnection)

	groupSelect = widget.NewSelect(selectorOptions, func(value string) {
		if value == "" {
			return
		}
		selectedGroup = value
		ac.SelectedClashGroup = value
		if suppressSelectCallback {
			return
		}
		status.SetText(fmt.Sprintf("Selected group '%s'.", value))
		onLoadAndRefreshProxies()
	})
	groupSelect.PlaceHolder = "Select selector group"
	if selectedGroup != "" {
		suppressSelectCallback = true
		groupSelect.SetSelected(selectedGroup)
		suppressSelectCallback = false
	}

	topControls := container.NewVBox(
		ac.ApiStatusLabel,
		container.NewHBox(widget.NewLabel("Selector group:"), groupSelect),
		testAPIButton,
		widget.NewSeparator(),
		loadButton,
	)

	contentContainer := container.NewBorder(
		topControls,
		status,
		nil,
		nil,
		scrollContainer,
	)

	return contentContainer
}
