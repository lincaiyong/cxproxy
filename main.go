package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/lincaiyong/log"
	"github.com/lincaiyong/uniapi/service/monica"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

func handler(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			log.ErrorLog("unexpected error: %v", err)
			c.String(http.StatusInternalServerError, fmt.Sprintf("%v", err))
		}
	}()

	var req Request
	err := c.ShouldBindJSON(&req)
	if err != nil {
		var r any
		_ = c.ShouldBindJSON(&r)
		log.ErrorLog("invalid request: %v, %v", err, r)
		c.String(http.StatusBadRequest, fmt.Sprintf("bad request: %v", err))
		return
	}

	if !req.Stream {
		log.ErrorLog("invalid request: stream=false")
		c.String(http.StatusBadRequest, "bad request: stream=false")
		return
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Expose-Headers", "Content-Type")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	var sb strings.Builder
	q := req.Compose()
	//if i := strings.Index(q, "</tools>"); i != -1 {
	//	log.InfoLog("req: %s", q[i+8:])
	//} else {
	log.InfoLog("req: %s", q)

	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "stream unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	created := time.Now().Unix()

	log.InfoLog("model: %s", req.Model)
	monica.Init(os.Getenv("MONICA_SESSION_ID"))
	_, err = monica.ChatCompletion(req.Model, q, func(s string) {
		sb.WriteString(s)

		chunk := ChatCompletionChunk{
			ID: "chatcmpl-llm-router-openai", Object: "chat.completion.chunk", Created: created, Model: req.Model,
			Choices: []ChatCompletionChoice{{
				Index: 0,
				Delta: ChatMessageDelta{Role: "assistant", Content: s},
			}},
			SystemFingerprint: "fp-llm-router-openai",
		}
		writeSSE(w, chunk)
		flusher.Flush()
	})
	answer := sb.String()
	log.InfoLog("resp: %s", answer)

	toolCalls := extractToolUse(answer)
	if len(toolCalls) == 0 {
		end := ChatCompletionChunk{ID: "chatcmpl-llm-router-openai", Object: "chat.completion.chunk", Created: created, Model: req.Model,
			Choices: []ChatCompletionChoice{{
				Index:        0,
				Delta:        ChatMessageDelta{},
				FinishReason: "stop",
			}},
			SystemFingerprint: "fp-llm-router-openai",
		}
		writeSSE(w, end)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		c.String(http.StatusOK, "ok")
		return
	}

	for _, tool := range toolCalls {
		chunk := ChatCompletionChunk{
			ID: "chatcmpl-llm-router-openai", Object: "chat.completion.chunk", Created: created, Model: req.Model,
			Choices: []ChatCompletionChoice{{
				Index: 0,
				Delta: ChatMessageDelta{
					Role: "assistant",
					ToolCalls: []ToolCallResponse{
						{
							Id:   genToolCallId(),
							Type: "function",
							Function: ToolCallFunction{
								Name:      tool[0],
								Arguments: tool[1],
							},
						},
					},
				},
			}},
			SystemFingerprint: "fp-llm-router-openai",
		}
		writeSSE(w, chunk)
		flusher.Flush()
	}
	end := ChatCompletionChunk{ID: "chatcmpl-llm-router-openai", Object: "chat.completion.chunk", Created: created, Model: req.Model,
		Choices: []ChatCompletionChoice{{
			Index:        0,
			Delta:        ChatMessageDelta{},
			FinishReason: "tool_calls",
		}},
		SystemFingerprint: "fp-llm-router-openai",
	}
	writeSSE(w, end)
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
	c.String(http.StatusOK, "ok")
}

func genToolCallId() string {
	length := 24
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return "call_" + string(result)
}

func writeSSE(w http.ResponseWriter, v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", b)
}

func extractToolUse(s string) [][2]string {
	items := regexp.MustCompile(`(?s)<use tool="(.+?)">(.+?)</use>`).FindAllStringSubmatch(s, -1)
	ret := make([][2]string, 0)
	for _, item := range items {
		args := strings.TrimSpace(item[2])
		ret = append(ret, [2]string{item[1], args})
	}
	return ret
}

func main() {
	port := 9123
	logPath := "/tmp/ccproxy.log"
	if err := log.SetLogPath(logPath); err != nil {
		log.ErrorLog("fail to set log file path: %v", err)
		os.Exit(1)
	}
	log.InfoLog("cmd line: %s", strings.Join(os.Args, " "))
	log.InfoLog("log path: %v", logPath)
	log.InfoLog("port: %d", port)
	log.InfoLog("pid: %d", os.Getpid())
	wd, _ := os.Getwd()
	log.InfoLog("work dir: %s", wd)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.InfoLog("receive quit signal")
		os.Exit(0)
	}()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		start := time.Now()
		log.InfoLog(" %s | %s", c.Request.URL.Path, c.ClientIP())
		c.Next()
		log.InfoLog(" %s | %s | %v | %d", c.Request.URL.Path, c.ClientIP(), time.Since(start), c.Writer.Status())
	})
	router.POST("v1/chat/completions", handler)

	log.InfoLog("starting server at 0.0.0.0:%d", port)
	err := router.Run(fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.ErrorLog("fail to run http server: %v", err)
		os.Exit(1)
	}
}
