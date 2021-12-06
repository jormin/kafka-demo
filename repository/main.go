package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/Shopify/sarama"
	"github.com/rs/zerolog"
)

func main() {
	logger := GetLogger()
	servers := []string{
		"kafka-0.kafka-headless.devops.svc.cluster.local:9092",
		"kafka-1.kafka-headless.devops.svc.cluster.local:9092",
		"kafka-2.kafka-headless.devops.svc.cluster.local:9092",
	}
	topic := "order"
	consumer, _ := sarama.NewConsumer(servers, nil)
	partition := int32(GetPodNo())
	pc, _ := consumer.ConsumePartition(topic, partition, sarama.OffsetOldest)
	for msg := range pc.Messages() {
		m := map[string]interface{}{
			"partition": partition,
			"offset":    msg.Offset,
			"key":       msg.Key,
			"value":     fmt.Sprintf("%s", msg.Value),
		}
		b, _ := json.Marshal(m)
		logger.Info().Msg(fmt.Sprintf("%s", b))
	}
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
