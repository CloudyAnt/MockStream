package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

type LogEntry struct {
	Timestamp string
	Summary   string
	Request   *http.Request
	Response  *http.Response
	Body      string
}

var (
	configMutex sync.RWMutex
	appConfig   Config
	server      *http.Server
	defaultPort = 10010
	portPicker  *PortPicker

	// --- logs ---
	logMutex    sync.RWMutex
	requestLogs []LogEntry
	maxLogs     = 100
	logList     *widget.List
)

// ResponseRecorder is a custom implementation of http.ResponseWriter that records the response
type ResponseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		ResponseWriter: w,
		body:           bytes.NewBuffer(nil),
	}
}

func (r *ResponseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *ResponseRecorder) Status() int {
	if r.statusCode == 0 {
		return http.StatusOK
	}
	return r.statusCode
}

func (r *ResponseRecorder) Body() *bytes.Buffer {
	return r.body
}

func main() {
	myApp := app.New()
	myApp.SetIcon(ResourceAppIconPng)
	window := myApp.NewWindow("OpenAI Mock Server")
	window.SetIcon(ResourceAppIconPng)

	// GUI
	backendEntry := widget.NewEntry()
	backendEntry.SetPlaceHolder("Input proxy url(.e.g. http://localhost:3001)")
	backendEntry.SetText("http://localhost:3001")
	backendEntry.Wrapping = fyne.TextWrapWord

	contentEntry := widget.NewMultiLineEntry()
	contentEntry.SetPlaceHolder("Input content")
	contentEntry.SetText("Hello, I am a mock server.")
	contentScroll := container.NewScroll(contentEntry)
	contentScroll.SetMinSize(fyne.NewSize(380, 200))

	thinkingEntry := widget.NewMultiLineEntry()
	thinkingEntry.SetPlaceHolder("Input reasoning content")
	thinkingEntry.SetText("I am thinking...")
	thinkingScroll := container.NewScroll(thinkingEntry)
	thinkingScroll.SetMinSize(fyne.NewSize(380, 200))

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
	createHeader := func(text string) *widget.Label {
		header := widget.NewLabel(text)
		header.TextStyle = fyne.TextStyle{Bold: true}
		header.Alignment = fyne.TextAlignLeading
		return header
	}

	// LAYOUT
	form := container.NewVBox(
		createHeader("Proxy Configuration"),
		container.NewPadded(backendEntry),
		container.NewHBox(
			container.NewPadded(mockSwitch),
			container.NewPadded(rawModeSwitch),
		),
		createHeader("Mock Functions"),
		container.NewPadded(mockFunctions),
		createHeader("Mock Thinking"),
		container.NewPadded(thinkingScroll),
		createHeader("Mock Content"),
		container.NewPadded(contentScroll),
	)

	logList = widget.NewList(
		func() int {
			logMutex.RLock()
			defer logMutex.RUnlock()
			return len(requestLogs)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			logMutex.RLock()
			log := requestLogs[id]
			logMutex.RUnlock()

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

	logList.OnSelected = func(id widget.ListItemID) {
		logMutex.RLock()
		log := requestLogs[id]
		logMutex.RUnlock()

		var details strings.Builder
		details.WriteString(fmt.Sprintf("Time: %s\n\n", log.Timestamp))

		// Request details
		details.WriteString("=== Request ===\n")
		if log.Request != nil {
			details.WriteString(fmt.Sprintf("Method: %s\n", log.Request.Method))
			details.WriteString(fmt.Sprintf("URL: %s\n", log.Request.URL.String()))
			details.WriteString("Headers:\n")
			for k, v := range log.Request.Header {
				details.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
			}
		} else {
			details.WriteString("No request information available\n")
		}

		// Response details
		details.WriteString("\n=== Response ===\n")
		if log.Response != nil {
			details.WriteString(fmt.Sprintf("Status: %s\n", log.Response.Status))
			details.WriteString("Headers:\n")
			for k, v := range log.Response.Header {
				details.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
			}
		} else {
			details.WriteString("No response information available\n")
		}

		// Body
		if log.Body != "" {
			details.WriteString("\n=== Body ===\n")
			details.WriteString(log.Body)
		}

		// Create a selectable text entry with better styling
		textEntry := widget.NewMultiLineEntry()
		textEntry.SetText(details.String())
		textEntry.Wrapping = fyne.TextWrapWord
		textEntry.Disable()

		// Create a scroll container for the text
		scroll := container.NewScroll(textEntry)
		scroll.SetMinSize(fyne.NewSize(500, 400))

		// Create a custom dialog with the scrollable text
		dialog.ShowCustom("Log Details", "Close", scroll, window)
	}

	logScroll := container.NewScroll(logList)
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

	portPicker = NewPortPicker("server port", defaultPort)
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
				Port:          portPicker.GetPort(),
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
	if mockingFunctions != nil && !strings.Contains(mockingFunctions[0], funcName) {
		logWithRequest(fmt.Sprintf("Not mocking function: %s", funcName), r, "")
		handleProxy(w, r)
		return
	}

	rawMode := appConfig.RawMode
	configMutex.RLock()
	thinking := appConfig.MockThinking
	content := appConfig.MockContent
	configMutex.RUnlock()

	summary := fmt.Sprintf("Mocking function: %s", funcName)
	logWithRequest(summary, r, fmt.Sprintf("Thinking: %s\nContent: %s\nRawMode: %t", thinking, content, rawMode))

	handleMockStream0(w, thinking, "reasoning_content", rawMode)
	handleMockStream0(w, content, "content", rawMode)
	fmt.Fprintf(w, "data: %s\n", "[DONE]")
	w.(http.Flusher).Flush()
}

func handleMockStream0(w http.ResponseWriter, content, key string, rawMode bool) {
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
		time.Sleep(200 * time.Millisecond)
	}
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	targetURL := appConfig.BackendURL
	configMutex.RUnlock()

	if targetURL == "" {
		http.Error(w, "Proxy URL is not set", http.StatusBadGateway)
		logWithRequest("Proxy URL is not set", r, "")
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

	// Create a response recorder to capture the response
	recorder := NewResponseRecorder(w)

	// Proxy the request
	logEntry := logWithRequest(fmt.Sprintf("Proxying request: %s", r.URL.String()), r, "")
	proxy.ServeHTTP(recorder, r)

	// Create a response object for logging
	resp := &http.Response{
		StatusCode: recorder.Status(),
		Header:     recorder.Header(),
		Body:       nil,
	}

	// Log the response
	logEntry.Response = resp
}

func logWithRequest(log string, req *http.Request, body string) *LogEntry {
	logMutex.Lock()
	defer logMutex.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Summary:   log,
		Request:   req,
		Body:      body,
	}
	requestLogs = append([]LogEntry{entry}, requestLogs...)
	if len(requestLogs) > maxLogs {
		requestLogs = requestLogs[:maxLogs]
	}
	if logList != nil {
		fyne.Do(logList.Refresh)
	}
	return &entry
}
