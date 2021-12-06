Kafka Demo
=====

[![Go Report Card](https://goreportcard.com/badge/github.com/jormin/kafka-demo)](https://goreportcard.com/report/github.com/jormin/kafka-demo)
[![](https://img.shields.io/badge/version-v1.0.0-success.svg)](https://github.com/jormin/kafka-demo)

[TOC]

### 1. 概述

这个项目是 Go 实现的 Kafka Demo，主要功能是模拟商城的下单流程，其中下单后订单服务会向 Kafka 生成一条信息， 仓储服务会从 Kafka 消费消息进行打包发货等处理，统计服务会从 Kafka 消费消息进行订单的统计。

这个 Demo 的重点在于 Kafka 消息的生产和消费，因此各个服务端的代码都省略，仅打印相关的日志信息用于校验数据。

整个项目的大体流程为：

1. 用户端调用 Api 网关的下单接口。
2. 网关层生成随机用户ID及订单金额，提交给订单服务的下单接口，订单服务返回生成的订单信息，网关层记录日志并给用户发送响应信息。
3. 订单服务接收到网关层请求后生成订单信息并将订单信息写入 Kafka，写入的主题分区从 hostname 中获取，之后记录日志。
4. 仓储服务从 Kafka 消费消息，消费的主题分区从 hostname 获取，之后记录日志。
5. 统计服务从 Kafka 消费改主题下的所有分区的消息，之后记录日志。

详细的流程图如下：

![1](https://blog.cdn.lerzen.com/img/20211206155214.png)

### 2. 资源清单

#### 2.1 硬件清单

| 节点   | 角色         | IP            | 配置      | Label               |
| ------ | ------------ | ------------- | --------- | ------------------- |
| master | master, etcd | 192.168.1.100 | 4核4G50G  | usefulness=schedule |
| node1  | worker       | 192.168.1.101 | 8核8G100G | usefulness=devops   |
| node2  | worker       | 192.168.1.102 | 8核8G100G | usefulness=demo     |
| node3  | worker       | 192.168.1.103 | 8核8G100G | usefulness=business |

#### 2.2 部署清单

本次部署的所有资源都在命名空间 **kafka-demo** 下，所有 Pod 都部署在 node3 节点上。

1. Api 网关层 - gateway：部署为 Deployment，启动 10 个 Pod
2. 订单服务 - order：部署为 Statefulset，启动 3 个 Pod
3. 仓储服务 - repository：部署为  Statefulset，启动 3 个 Pod
4. 统计服务 - statistics：部署为  Deployment，启动 1 个 Pod
5. Kafka：使用 [helm安装zookeeper和kafka](https://blog.lerzen.com/post/helm%E5%AE%89%E8%A3%85zookeeper%E5%92%8Ckafka/) 中部署好的 Kafka，使用的 Topic 为 **order**，有三个分区，两个备份

#### 2.3 域名清单

| 域名                          | 类型         | IP            | Service |
| ----------------------------- | ------------ | ------------- | ------- |
| kafka-demo.local.com                | IngressRoute | 192.168.1.100 | gateway |
| order.kafka-demo.svc.clauster.local | DNS          | -----         | order   |

### 3. 部署

#### 3.1 域名写入 hosts 信息

```shell
cat >> /etc/hosts <<EOF
192.168.1.101 kafka-demo.local.com
EOF
```

#### 3.2 部署Api网关层

#### 3.3 部署订单服务

#### 3.4 部署仓储服务

#### 3.5 部署统计服务

### 4. 测试

### 5. 总结

License
-------

Under the [MIT](./LICENSE) License