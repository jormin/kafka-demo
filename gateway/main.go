package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/jormin/gobox/id"
	"github.com/rs/zerolog"
)

func main() {
	http.HandleFunc(
		"/", func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, "gateway")
		},
	)
	http.HandleFunc("/submit-order", SubmitOrder)
	_ = http.ListenAndServe(":18080", nil)
}

// SubmitOrder 提交订单
func SubmitOrder(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	partition := r.Form.Get("partition")
	url := fmt.Sprintf("http://order-%s.order.kafka-demo.svc.cluster.local/submit-order", partition)
	m := map[string]interface{}{
		"time":    time.Now().Unix(),
		"user_id": id.NewID(),
		"money":   rand.Intn(1000),
	}
	body, _ := json.Marshal(m)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)
	content := fmt.Sprintf("%s", b)
	logger := GetLogger()
	logger.Info().Msg(content)
	_, _ = fmt.Fprint(w, content)
}

// GetLogger 获取Logger
func GetLogger() zerolog.Logger {
	return zerolog.New(
		zerolog.MultiLevelWriter(
			os.Stdout,
		),
	).With().Timestamp().Logger()
}
