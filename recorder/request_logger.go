package recorder

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type RequestLogEntry struct {
	Timestamp string
	Summary   string
	Request   *http.Request
	Response  *http.Response
	Body      string
}

type RequestLogger struct {
	mutex       sync.RWMutex
	requestLogs []RequestLogEntry
	maxLogs     int
	logList     *widget.List
}

func NewRequestLogger(maxLogs int) *RequestLogger {
	return &RequestLogger{
		maxLogs: maxLogs,
	}
}

func (l *RequestLogger) SetLogList(logList *widget.List) {
	l.logList = logList
}

func (l *RequestLogger) GetLogCount() int {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return len(l.requestLogs)
}

func (l *RequestLogger) GetLog(index int) RequestLogEntry {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.requestLogs[index]
}

func (l *RequestLogger) LogWithRequest(log string, req *http.Request, body string) *RequestLogEntry {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	entry := RequestLogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Summary:   log,
		Request:   req,
		Body:      body,
	}
	l.requestLogs = append([]RequestLogEntry{entry}, l.requestLogs...)
	if len(l.requestLogs) > l.maxLogs {
		l.requestLogs = l.requestLogs[:l.maxLogs]
	}
	if l.logList != nil {
		fyne.Do(l.logList.Refresh)
	}
	return &entry
}

func (l *RequestLogger) FormatLogDetails(log RequestLogEntry) string {
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

	return details.String()
}
