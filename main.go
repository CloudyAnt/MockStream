package main

import (
	"encoding/json"
	"fmt"
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
	BackendURL   string
	MockContent  string
	MockThinking string
	Running      bool
}

var (
	configMutex sync.RWMutex
	appConfig   Config
	server      *http.Server
	port        = 10086
)

func main() {
	myApp := app.New()
	myApp.SetIcon(ResourceAppIconPng)
	window := myApp.NewWindow("OpenAI Mock Server")
	window.SetIcon(ResourceAppIconPng)

	// GUI
	backendEntry := widget.NewEntry()
	backendEntry.SetPlaceHolder("Input proxy url(.e.g. http://api.openai.com)")

	contentEntry := widget.NewMultiLineEntry()
	contentEntry.SetPlaceHolder("Input content")
	contentScroll := container.NewScroll(contentEntry)
	contentScroll.SetMinSize(fyne.NewSize(380, 200))

	thinkingEntry := widget.NewMultiLineEntry()
	thinkingEntry.SetPlaceHolder("Input reasoning content")
	thinkingScroll := container.NewScroll(thinkingEntry)
	thinkingScroll.SetMinSize(fyne.NewSize(380, 200))

	statusLabel := widget.NewLabel("Server Status: Not Running")
	startButton := widget.NewButton("Start Server", nil)

	// LAYOUT
	form := container.NewVBox(
		widget.NewLabel("Proxy URL"),
		backendEntry,
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
		fmt.Printf(">> %p, %s\n", &appConfig, appConfig.MockThinking)
		configMutex.Unlock()
	}

	startButton.OnTapped = func() {
		configMutex.Lock()
		defer configMutex.Unlock()

		if appConfig.Running {
			server.Close()
			appConfig.Running = false
			statusLabel.SetText("Server Status: Stopped")
			startButton.SetText("Start Server")
		} else {
			appConfig = Config{
				BackendURL:   backendEntry.Text,
				MockContent:  contentEntry.Text,
				MockThinking: thinkingEntry.Text,
				Running:      true,
			}
			startServer()
			statusLabel.SetText(fmt.Sprintf("Server Status: Started (Port:%d)", port))
			startButton.SetText("Stop Server")
		}
	}

	window.SetContent(container.NewVBox(
		form,
		statusLabel,
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
		Addr:    ":" + strconv.Itoa(port),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
}

func handleMockStream(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	fmt.Printf(">>> %p, %s\n", &appConfig, appConfig.MockThinking)
	thinking := appConfig.MockThinking
	content := appConfig.MockContent
	configMutex.RUnlock()
	fmt.Printf("Mocking thinking: %s\nMocking content: %s\n", thinking, content)

	// 设置公共响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher := w.(http.Flusher)

	if thinking != "" {
		reasoningData := map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"delta": map[string]string{
						"reasoning_content": thinking,
					},
				},
			},
		}
		if jsonData, err := json.Marshal(reasoningData); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
			time.Sleep(300 * time.Millisecond) // 保持与后续内容的时间间隔
		}
	}

	// Spilt the mock text into chunks
	chunks := strings.SplitAfter(content, "\n")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}

		data := map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"delta": map[string]string{
						"content": chunk,
					},
				},
			},
		}

		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		w.(http.Flusher).Flush()
		time.Sleep(500 * time.Millisecond)
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

	// Maintain the same host
	r.URL.Host = target.Host
	r.URL.Scheme = target.Scheme
	r.Header.Set("Host", target.Host)
	r.Host = target.Host

	// Proxy the request
	proxy.ServeHTTP(w, r)
}
