package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"mock-stream/recorder"
	"mock-stream/ui"
)

type Config struct {
	BackendURL    string
	MockContent   string
	MockThinking  string
	MockFunctions string
	Running       bool
	MockEnabled   bool
	RawMode       bool // return raw line instead of "data: {...}"
	Port          int
}

var (
	configMutex sync.RWMutex
	appConfig   Config
	server      *http.Server
	defaultPort = 10010
	portPicker  *ui.NumberPicker

	// --- logs ---
	requestLogger *recorder.RequestLogger
	reqLogList    *widget.List
)

func main() {
	myApp := app.New()
	myApp.SetIcon(ResourceAppIconPng)
	window := myApp.NewWindow("OpenAI Mock Server")
	window.SetIcon(ResourceAppIconPng)

	// Initialize logger
	requestLogger = recorder.NewRequestLogger(100)

	// GUI
	backendEntry := widget.NewEntry()
	backendEntry.SetPlaceHolder("Input proxy url(.e.g. http://localhost:3001)")
	backendEntry.SetText("http://localhost:3001")

	contentEntry := widget.NewMultiLineEntry()
	contentEntry.SetPlaceHolder("Input content (Click ⇥ button to insert tab)")
	contentEntry.SetText("Hello, I am a mock server.")
	contentScroll := container.NewScroll(contentEntry)
	contentScroll.SetMinSize(fyne.NewSize(380, 200))

	tabButton := widget.NewButton("Insert Tab", func() {
		contentEntry.TypedRune('⇥')
	})
	contentContainer := container.NewVBox(contentScroll)

	thinkingEntry := widget.NewMultiLineEntry()
	thinkingEntry.SetPlaceHolder("Input reasoning content (Click ⇥ button to insert tab)")
	thinkingEntry.SetText("I am thinking...")
	thinkingScroll := container.NewScroll(thinkingEntry)
	thinkingScroll.SetMinSize(fyne.NewSize(380, 200))

	thinkingTabButton := widget.NewButton("Insert Tab", func() {
		thinkingEntry.TypedRune('⇥')
	})
	thinkingContainer := container.NewVBox(thinkingScroll)

	statusLabel := widget.NewLabel("Server Status: Not Running")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	startButton := widget.NewButton("Start Server ▶️", nil)
	startButton.Importance = widget.HighImportance

	mockSwitch := widget.NewCheck("Enable Mock", func(checked bool) {
		configMutex.Lock()
		appConfig.MockEnabled = checked
		configMutex.Unlock()
	})
	mockSwitch.SetChecked(true)

	rawModeSwitch := widget.NewCheck("Raw Mode", func(checked bool) {
		configMutex.Lock()
		appConfig.RawMode = checked
		configMutex.Unlock()
	})
	rawModeSwitch.SetChecked(false)

	mockFunctions := widget.NewEntry()
	mockFunctions.SetPlaceHolder("Input mock functions(.e.g. chat,codebase)")
	mockFunctions.SetText("chat")

	// Create section headers with custom styling
	createHeader := func(text string, tabButton *widget.Button) *fyne.Container {
		header := widget.NewLabel(text)
		header.TextStyle = fyne.TextStyle{Bold: true}
		header.Alignment = fyne.TextAlignLeading
		if tabButton != nil {
			return container.NewHBox(header, tabButton)
		}
		return container.NewHBox(header)
	}

	// LAYOUT
	form := container.NewVBox(
		createHeader("Proxy Configuration", nil),
		container.NewPadded(backendEntry),
		container.NewHBox(
			container.NewPadded(mockSwitch),
			container.NewPadded(rawModeSwitch),
		),
		createHeader("Mock Functions", nil),
		container.NewPadded(mockFunctions),
		createHeader("Mock Thinking", thinkingTabButton),
		container.NewPadded(thinkingContainer),
		createHeader("Mock Content", tabButton),
		container.NewPadded(contentContainer),
	)

	reqLogList = widget.NewList(
		func() int {
			return requestLogger.GetLogCount()
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("Template")
			label.TextStyle = fyne.TextStyle{Monospace: true}
			return label
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			log := requestLogger.GetLog(id)

			// Format the list item text
			var status string
			if log.Response != nil {
				status = fmt.Sprintf("[%d]", log.Response.StatusCode)
			} else {
				status = "[---]"
			}

			// Get request info
			var method, path string
			if log.Request != nil {
				method = log.Request.Method
				path = log.Request.URL.Path
			} else {
				method = "---"
				path = "---"
			}

			item.(*widget.Label).SetText(fmt.Sprintf("%s %s %s %s",
				log.Timestamp, status, method, path))
		},
	)

	requestLogger.SetLogList(reqLogList)

	reqLogList.OnSelected = func(id widget.ListItemID) {
		log := requestLogger.GetLog(id)

		// Create a selectable text entry with better styling
		textEntry := widget.NewMultiLineEntry()
		textEntry.SetText(requestLogger.FormatLogDetails(log))
		textEntry.Wrapping = fyne.TextWrapWord
		textEntry.TextStyle = fyne.TextStyle{Monospace: true}
		textEntry.Disable()

		// Create a scroll container for the text
		scroll := container.NewScroll(textEntry)
		scroll.SetMinSize(fyne.NewSize(500, 400))

		// Create a custom dialog with the scrollable text
		d := dialog.NewCustom("Log Details", "Close", scroll, window)
		d.SetOnClosed(func() {
			// Reset the selection after dialog is closed
			reqLogList.Unselect(id)
		})
		d.Show()
	}

	logScroll := container.NewScroll(reqLogList)
	logScroll.SetMinSize(fyne.NewSize(380, 200))

	// EVENT HANDLER
	backendEntry.OnChanged = func(text string) {
		configMutex.Lock()
		appConfig.BackendURL = text
		configMutex.Unlock()
	}

	contentEntry.OnChanged = func(text string) {
		configMutex.Lock()
		appConfig.MockContent = text
		configMutex.Unlock()
	}

	thinkingEntry.OnChanged = func(text string) {
		configMutex.Lock()
		appConfig.MockThinking = text
		configMutex.Unlock()
	}

	mockFunctions.OnChanged = func(text string) {
		configMutex.Lock()
		appConfig.MockFunctions = text
		configMutex.Unlock()
	}

	portPicker = ui.NewPortPicker("server port", defaultPort)
	startButton.OnTapped = func() {
		configMutex.Lock()
		defer configMutex.Unlock()

		if appConfig.Running {
			server.Close()
			appConfig.Running = false
			statusLabel.SetText("Server Status: Stopped")
			startButton.SetText("Start Server ▶️")
			portPicker.Enable()
		} else {
			appConfig = Config{
				BackendURL:    backendEntry.Text,
				MockContent:   contentEntry.Text,
				MockThinking:  thinkingEntry.Text,
				MockEnabled:   mockSwitch.Checked,
				RawMode:       rawModeSwitch.Checked,
				MockFunctions: mockFunctions.Text,
				Port:          portPicker.GetValue(),
				Running:       true,
			}
			startServer()
			statusLabel.SetText(fmt.Sprintf("Server Status: Started (Port:%d)", appConfig.Port))
			startButton.SetText("Stop Server 🔴")
			portPicker.Disable()
		}
	}

	mainPage := container.NewVBox(
		container.NewPadded(form),
		container.NewPadded(statusLabel),
		container.NewPadded(portPicker.GetUI()),
		container.NewPadded(startButton),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("Mock", mainPage),
		container.NewTabItem("Logs", logScroll),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	window.SetContent(tabs)
	window.Resize(fyne.NewSize(600, 800))
	window.ShowAndRun()
}

func startServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat/completions", handleMockStream)
	mux.HandleFunc("/", handleProxy)

	server = &http.Server{
		Addr:    ":" + strconv.Itoa(appConfig.Port),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
}

func handleMockStream(w http.ResponseWriter, r *http.Request) {
	if !appConfig.MockEnabled {
		handleProxy(w, r)
		return
	}
	mockingFunctions := strings.Split(appConfig.MockFunctions, ",")
	funcName := r.Header.Get("FunctionName")

	// Check if the function name is in the list of mocking functions
	shouldMock := false
	for _, mockFunc := range mockingFunctions {
		if strings.TrimSpace(mockFunc) == funcName {
			shouldMock = true
			break
		}
	}

	if !shouldMock {
		handleProxy(w, r)
		return
	}

	rawMode := appConfig.RawMode
	configMutex.RLock()
	thinking := appConfig.MockThinking
	content := appConfig.MockContent
	configMutex.RUnlock()

	summary := fmt.Sprintf("Mocking function: %s", funcName)
	requestLogger.LogWithRequest(summary, r, fmt.Sprintf("Thinking: %s\nContent: %s\nRawMode: %t", thinking, content, rawMode))

	handleMockStream0(w, thinking, "reasoning_content", rawMode)
	handleMockStream0(w, content, "content", rawMode)
	fmt.Fprintf(w, "data: %s\n", "[DONE]")
	w.(http.Flusher).Flush()
}

func handleMockStream0(w http.ResponseWriter, content, key string, rawMode bool) {
	content = strings.ReplaceAll(content, "⇥", "\t")
	chunks := strings.SplitAfter(content, "\n")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	count := 0
	for _, chunk := range chunks {
		if chunk == "" {
			continue
		}
		ch := chunk
		if rawMode {
			fmt.Fprintf(w, "%s\n", ch)
		} else {
			data := map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta": map[string]string{
							key: ch,
						},
					},
				},
			}

			jsonData, _ := json.Marshal(data)
			fmt.Fprintf(w, "data: %s\n", jsonData)
		}

		w.(http.Flusher).Flush()
		count++
		time.Sleep(20 * time.Millisecond)
	}
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	targetURL := appConfig.BackendURL
	configMutex.RUnlock()

	if targetURL == "" {
		http.Error(w, "Proxy URL is not set", http.StatusBadGateway)
		requestLogger.LogWithRequest("Proxy URL is not set", r, "")
		return
	}

	target, _ := url.Parse(targetURL)
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customize the proxy's transport
	proxy.Transport = &http.Transport{
		Proxy: nil, // Disable system proxy
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true, // Disable compression for streaming
	}

	// Maintain the same host
	r.URL.Host = target.Host
	r.URL.Scheme = target.Scheme
	r.Header.Set("Host", target.Host)
	r.Host = target.Host

	// Set streaming headers only for streaming endpoints
	if r.URL.Path == "/chat/completions" {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering if behind nginx
	}

	// Proxy the request
	logEntry := requestLogger.LogWithRequest(fmt.Sprintf("Proxying request: %s", r.URL.String()), r, "")

	// Create a response recorder to capture the response
	recorder := recorder.NewResponseRecorder(w, logEntry)
	proxy.ServeHTTP(recorder, r)

	// Create a response object for logging
	resp := &http.Response{
		StatusCode: recorder.Status(),
		Header:     recorder.Header(),
		Body:       io.NopCloser(recorder.Body()),
	}

	// Log the response
	logEntry.Response = resp
}
