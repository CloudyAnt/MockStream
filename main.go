package main

import (
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
	"fyne.io/fyne/v2/widget"
)

type Config struct {
	BackendURL    string
	MockContent   string
	MockThinking  string
	MockFunctions string
	Running       bool
	MockEnabled   bool
	Port          int
}

var (
	configMutex sync.RWMutex
	appConfig   Config
	server      *http.Server
	defaultPort = 10086
	portPicker  *PortPicker
)

func main() {
	myApp := app.New()
	myApp.SetIcon(ResourceAppIconPng)
	window := myApp.NewWindow("OpenAI Mock Server")
	window.SetIcon(ResourceAppIconPng)

	// GUI
	backendEntry := widget.NewEntry()
	backendEntry.SetPlaceHolder("Input proxy url(.e.g. http://localhost:3001)")
	backendEntry.SetText("http://localhost:3001")

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
	startButton := widget.NewButton("Start Server", nil)

	mockSwitch := widget.NewCheck("Enable Mock", func(checked bool) {
		configMutex.Lock()
		appConfig.MockEnabled = checked
		configMutex.Unlock()
	})
	mockSwitch.SetChecked(true)

	mockFunctions := widget.NewEntry()
	mockFunctions.SetPlaceHolder("Input mock functions(.e.g. chat,codebase)")
	mockFunctions.SetText("chat")

	// LAYOUT
	form := container.NewVBox(
		widget.NewLabel("Proxy URL"),
		backendEntry,
		mockSwitch,
		widget.NewLabel("Mock Functions"),
		mockFunctions,
		widget.NewLabel("Mock Thinking"),
		thinkingScroll,
		widget.NewLabel("Mock Content"),
		contentScroll,
	)

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
			startButton.SetText("Start Server")
			portPicker.GetUI().Show()
		} else {
			appConfig = Config{
				BackendURL:    backendEntry.Text,
				MockContent:   contentEntry.Text,
				MockThinking:  thinkingEntry.Text,
				MockEnabled:   mockSwitch.Checked,
				MockFunctions: mockFunctions.Text,
				Port:          portPicker.GetPort(),
				Running:       true,
			}
			startServer()
			statusLabel.SetText(fmt.Sprintf("Server Status: Started (Port:%d)", appConfig.Port))
			startButton.SetText("Stop Server")
			portPicker.GetUI().Hide()
		}
	}

	window.SetContent(container.NewVBox(
		form,
		statusLabel,
		portPicker.GetUI(),
		startButton,
	))
	window.Resize(fyne.NewSize(500, 600))
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
		fmt.Printf("Mocking disabled\n")
		handleProxy(w, r)
		return
	}
	mockingFunctions := strings.Split(appConfig.MockFunctions, ",")
	funcName := r.Header.Get("Functionname")
	if mockingFunctions != nil && !strings.Contains(mockingFunctions[0], funcName) {
		fmt.Printf("Not mocking function: %s\n", funcName)
		handleProxy(w, r)
		return
	}

	configMutex.RLock()
	thinking := appConfig.MockThinking
	content := appConfig.MockContent
	configMutex.RUnlock()
	fmt.Printf("Mocking function: %s\nMocking thinking: %s\nMocking content: %s\n", funcName, thinking, content)

	handleMockStream0(w, thinking, "reasoning_content")
	handleMockStream0(w, content, "content")
}

func handleMockStream0(w http.ResponseWriter, content, key string) {
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
		w.(http.Flusher).Flush()
		count++
		time.Sleep(500 * time.Millisecond)

	}
	if count > 0 {
		fmt.Fprintf(w, "data: %s\n", "[DONE]")
		w.(http.Flusher).Flush()
	}
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	targetURL := appConfig.BackendURL
	configMutex.RUnlock()

	if targetURL == "" {
		http.Error(w, "Proxy URL is not set", http.StatusBadGateway)
		return
	}

	target, _ := url.Parse(targetURL)
	proxy := httputil.NewSingleHostReverseProxy(target)
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
	}

	// Maintain the same host
	r.URL.Host = target.Host
	r.URL.Scheme = target.Scheme
	r.Header.Set("Host", target.Host)
	r.Host = target.Host

	// Proxy the request
	fmt.Printf("Proxying request: %s\n", r.URL.String())
	proxy.ServeHTTP(w, r)
}
