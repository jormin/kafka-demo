package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/Shopify/sarama"
	"github.com/jormin/gobox/id"
	"github.com/rs/zerolog"
)

// producer Kafka生产者
var producer sarama.SyncProducer

func main() {
	InitKafka()
	http.HandleFunc(
		"/", func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, "order")
		},
	)
	http.HandleFunc("/submit-order", SubmitOrder)
	_ = http.ListenAndServe(":80", nil)
}

// SubmitOrder 提交订单
func SubmitOrder(w http.ResponseWriter, r *http.Request) {
	// 生成订单信息
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("read body err, %v\n", err)
		return
	}
	params := make(map[string]interface{})
	_ = json.Unmarshal(body, &params)
	params["order_id"] = id.NewID()
	b, _ := json.Marshal(params)
	content := fmt.Sprintf("%s", b)

	//  记录日志
	logger := GetLogger()
	logger.Info().Msg(content)

	// Kafka生产消息
	msg := &sarama.ProducerMessage{}
	msg.Topic = "order"
	msg.Partition = int32(GetPodNo())
	msg.Value = sarama.StringEncoder(content)
	_, _, _ = producer.SendMessage(msg)

	// 响应客户端
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

// GetPodNo 获取Pod编号
func GetPodNo() int {
	host, _ := os.Hostname()
	host = "order-1"
	pattern := regexp.MustCompile(`.*(\d+)`)
	res := pattern.FindAllStringSubmatch(host, -1)
	no, _ := strconv.Atoi(res[0][1])
	return no
}

// InitKafka 初始化Kafka
func InitKafka() {
	servers := []string{
		"kafka-0.kafka-headless.devops.svc.cluster.local:9092",
		"kafka-1.kafka-headless.devops.svc.cluster.local:9092",
		"kafka-2.kafka-headless.devops.svc.cluster.local:9092",
	}
	sarama.Logger = log.New(os.Stderr, "[Sarama] ", log.LstdFlags)
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Return.Successes = true
	cfg.Producer.Partitioner = sarama.NewManualPartitioner
	// 生产消息
	producer, _ = sarama.NewSyncProducer(servers, cfg)
}
