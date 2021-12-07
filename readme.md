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

#### 2.1 硬件

| 节点   | 角色         | IP            | 配置      | Label               |
| ------ | ------------ | ------------- | --------- | ------------------- |
| master | master, etcd | 192.168.1.100 | 4核4G50G  | usefulness=schedule |
| node1  | worker       | 192.168.1.101 | 8核8G100G | usefulness=devops   |
| node2  | worker       | 192.168.1.102 | 8核8G100G | usefulness=demo     |
| node3  | worker       | 192.168.1.103 | 8核8G100G | usefulness=business |

#### 2.2 部署方案

本次部署的所有资源都在命名空间 **kafka-demo** 下，所有 Pod 都部署在 node3 节点上。

1. Api 网关层 - gateway：部署为 Deployment，启动 10 个 Pod
2. 订单服务 - order：部署为 Statefulset，启动 3 个 Pod
3. 仓储服务 - repository：部署为  Statefulset，启动 3 个 Pod
4. 统计服务 - statistics：部署为  Deployment，启动 1 个 Pod
5. Kafka：使用 [helm安装zookeeper和kafka](https://blog.lerzen.com/post/helm%E5%AE%89%E8%A3%85zookeeper%E5%92%8Ckafka/) 中部署好的 Kafka，使用的 Topic 为 **order**，有三个分区，两个备份

#### 2.3 域名

| 域名                          | 类型         | IP            | Service |
| ----------------------------- | ------------ | ------------- | ------- |
| kafka-demo.local.com                | IngressRoute | 192.168.1.100 | gateway |
| order.kafka-demo.svc.cluster.local | DNS          | -----         | order   |

#### 2.4 镜像

打包的镜像我是放在自己的 harbor 上，也可以放在 docker hub 或者 托管在 腾讯云 或者 阿里云等等，对应的需要修改 **build.sh** 和 **k8s/deploy/*.yaml** 中的镜像地址以及拉取镜像的秘钥配置。

### 3. 部署

#### 3.1 域名写入 hosts 信息

```shell
cat >> /etc/hosts <<EOF
192.168.1.101 kafka-demo.local.com
EOF
```

#### 3.2 创建 Topic

部署服务之前，需要先创建一个名为 **order** 的 Topic，这里我们通过 kafka client pod 来创建

创建并连接 kafka client pod

```shell
root@master:~/kafka-demo/k8s/deploy# kubectl run kafka-client --restart='Never' --image docker.io/bitnami/kafka:2.8.1-debian-10-r57 --namespace devops --command -- sleep infinity
pod/kafka-client created

root@master:~/kafka-demo/k8s/deploy# kubectl exec --tty -i kafka-client --namespace devops -- bash
I have no name!@kafka-client:/$
```

连接上 kafka client pod 后，创建 Topic

```shell
I have no name!@kafka-client:/$ kafka-topics.sh --bootstrap-server kafka.devops.svc.cluster.local:9092 --list
__consumer_offsets

I have no name!@kafka-client:/$ kafka-topics.sh --bootstrap-server kafka.devops.svc.cluster.local:9092 --create --topic order --partitions 3 --replication-factor 2
Created topic order.
```

#### 3.3 部署Api网关层

Gateway 部署为 Deployment，启动10个Pod，需要配一个 Service 和 IngressRoute，使用域名 **kafka-demo.local.com** 访问，资源清单如下：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
  namespace: kafka-demo
spec:
  # 期望 pod 数量
  replicas: 10
  # 新创建的 pod 在运行指定秒数后才视为运行可用，配合就绪探针可以在滚动升级失败的时候阻止升级，避免部署出错的应用
  minReadySeconds: 2
  strategy:
    rollingUpdate:
      # 滚动升级过程中最多允许超出期望副本数的数量，比如期望3，maxSurge 配置为1，则最多存在4个pod，也可以配置百分比
      maxSurge: 1
      # 滚动升级过程中最多允许存在不可用的 pod 数量，配置为0表示升级过程中所有的 pod 都必须可用，即 pod 挨个替换，也可以配置百分比
      maxUnavailable: 0
  # 匹配器，匹配 pod 的方式
  selector:
    matchLabels:
      app: gateway
  template:
    metadata:
      name: gateway
      labels:
        app: gateway
    spec:
      imagePullSecrets:
        - name: harbor-jormin
      containers:
        - name: gateway
          image: harbor.wcxst.com/kafka-demo/gateway:latest
          # 就绪探针
          readinessProbe:
            # 执行周期，单位：秒
            periodSeconds: 1
            # 初始化延迟，单位：秒
            initialDelaySeconds: 1
            httpGet:
              path: /
              port: 80

---

kind: Service
apiVersion: v1
metadata:
  name: gateway
  namespace: kafka-demo
spec:
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  selector:
    app: gateway

---

apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: gateway
  namespace: kafka-demo
spec:
  entryPoints:
    - web
  routes:
    - match: Host(`kafka-demo.local.com`) && (PathPrefix(`/`)
      kind: Rule
      services:
        - name: gateway
          port: 80
```

#### 3.4 部署订单服务

Order 部署为 StatefulSet，启动3个Pod，需要配一个 Service ，使用内部域名 **order.kafka-demo.svc.clauster.local** 访问，资源清单如下：

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: order
  namespace: kafka-demo
spec:
  # 期望 pod 数量
  replicas: 3
  serviceName: "order"
  selector:
    matchLabels:
      app: order
  template:
    metadata:
      name: order
      labels:
        app: order
    spec:
      imagePullSecrets:
        - name: harbor-jormin
      containers:
        - name: order
          image: harbor.wcxst.com/kafka-demo/order:latest
          # 就绪探针
          readinessProbe:
            # 执行周期，单位：秒
            periodSeconds: 1
            # 初始化延迟，单位：秒
            initialDelaySeconds: 1
            httpGet:
              path: /
              port: 80

---

kind: Service
apiVersion: v1
metadata:
  name: order
  namespace: kafka-demo
spec:
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  selector:
    app: order
```

#### 3.5 部署仓储服务

Repository 部署为 StatefulSet，启动3个Pod，资源清单如下：

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: repository
  namespace: kafka-demo
spec:
  # 期望 pod 数量
  replicas: 3
  serviceName: "repository"
  selector:
    matchLabels:
      app: repository
  template:
    metadata:
      name: repository
      labels:
        app: repository
    spec:
      imagePullSecrets:
        - name: harbor-jormin
      containers:
        - name: repository
          image: harbor.wcxst.com/kafka-demo/repository:latest
          # 就绪探针
          readinessProbe:
            # 执行周期，单位：秒
            periodSeconds: 1
            # 初始化延迟，单位：秒
            initialDelaySeconds: 1
            exec:
              command:
                - ls
```

#### 3.6 部署统计服务

Statistics 服务部署为 Deployment，只需要启动1个Pod，资源清单如下：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: statistics
  namespace: kafka-demo
spec:
  # 期望 pod 数量
  replicas: 1
  strategy:
    rollingUpdate:
      # 滚动升级过程中最多允许超出期望副本数的数量，比如期望3，maxSurge 配置为1，则最多存在4个pod，也可以配置百分比
      maxSurge: 1
      # 滚动升级过程中最多允许存在不可用的 pod 数量，配置为0表示升级过程中所有的 pod 都必须可用，即 pod 挨个替换，也可以配置百分比
      maxUnavailable: 0
  # 匹配器，匹配 pod 的方式
  selector:
    matchLabels:
      app: statistics
  template:
    metadata:
      name: statistics
      labels:
        app: statistics
    spec:
      imagePullSecrets:
        - name: harbor-jormin
      containers:
        - name: statistics
          image: harbor.wcxst.com/kafka-demo/statistics:latest
          # 就绪探针
          readinessProbe:
            # 执行周期，单位：秒
            periodSeconds: 1
            # 初始化延迟，单位：秒
            initialDelaySeconds: 1
            exec:
              command:
                - ls
```

#### 3.7  部署结果

部署完成后，我们检查下 kafka-demo 下的资源列表

##### Deployment

```shell
root@master:~/kafka-demo/k8s/deploy# kubectl get deploy -n kafka-demo -o wide
NAME         READY   UP-TO-DATE   AVAILABLE   AGE     CONTAINERS   IMAGES                                          SELECTOR
gateway      10/10   10           10          2m37s   gateway      harbor.wcxst.com/kafka-demo/gateway:latest      app=gateway
statistics   1/1     1            1           2m37s   statistics   harbor.wcxst.com/kafka-demo/statistics:latest   app=statistics
```

##### StatefulSet

```shell
root@master:~/kafka-demo/k8s/deploy# kubectl get statefulset -n kafka-demo -o wide
NAME         READY   AGE     CONTAINERS   IMAGES
order        3/3     3m19s   order        harbor.wcxst.com/kafka-demo/order:latest
repository   3/3     3m19s   repository   harbor.wcxst.com/kafka-demo/repository:latest
```

##### Pod

```shell
root@master:~/kafka-demo/k8s/deploy# kubectl get pod -n kafka-demo
NAME                          READY   STATUS    RESTARTS   AGE
gateway-575f596c47-5jt6r      1/1     Running   0          3m53s
gateway-575f596c47-c6sf9      1/1     Running   0          3m53s
gateway-575f596c47-cglx9      1/1     Running   0          3m53s
gateway-575f596c47-dzjnb      1/1     Running   0          3m53s
gateway-575f596c47-fbrvg      1/1     Running   0          3m53s
gateway-575f596c47-hfk5n      1/1     Running   0          3m53s
gateway-575f596c47-rksk7      1/1     Running   0          3m53s
gateway-575f596c47-wdsxg      1/1     Running   0          3m53s
gateway-575f596c47-wvkw2      1/1     Running   0          3m53s
gateway-575f596c47-zvtkb      1/1     Running   0          3m53s
order-0                       1/1     Running   0          3m53s
order-1                       1/1     Running   0          3m30s
order-2                       1/1     Running   0          3m24s
repository-0                  1/1     Running   0          3m53s
repository-1                  1/1     Running   0          3m33s
repository-2                  1/1     Running   0          3m27s
statistics-5fb5557d68-8bzrc   1/1     Running   0          3m53s
```

##### Service

```shell
root@master:~/kafka-demo/k8s/deploy# kubectl get svc -n kafka-demo -o wide
NAME      TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE     SELECTOR
gateway   ClusterIP   10.233.36.207   <none>        80/TCP    4m49s   app=gateway
order     ClusterIP   10.233.17.102   <none>        80/TCP    4m49s   app=order

root@master:~/kafka-demo/k8s/deploy# kubectl describe svc gateway -n kafka-demo
Name:              gateway
Namespace:         kafka-demo
Labels:            <none>
Annotations:       <none>
Selector:          app=gateway
Type:              ClusterIP
IP Family Policy:  SingleStack
IP Families:       IPv4
IP:                10.233.36.207
IPs:               10.233.36.207
Port:              <unset>  80/TCP
TargetPort:        80/TCP
Endpoints:         10.233.92.125:80,10.233.92.126:80,10.233.92.127:80 + 7 more...
Session Affinity:  None
Events:            <none>

root@master:~/kafka-demo/k8s/deploy# kubectl describe svc order -n kafka-demo
Name:              order
Namespace:         kafka-demo
Labels:            <none>
Annotations:       <none>
Selector:          app=order
Type:              ClusterIP
IP Family Policy:  SingleStack
IP Families:       IPv4
IP:                10.233.17.102
IPs:               10.233.17.102
Port:              <unset>  80/TCP
TargetPort:        80/TCP
Endpoints:         10.233.92.137:80,10.233.92.139:80,10.233.92.141:80
Session Affinity:  None
Events:            <none>
```

##### IngressRoute

```shell
root@master:~/kafka-demo/k8s/deploy# kubectl get ingressroute -n kafka-demo -o wide
NAME      AGE
gateway   5m15s
```

### 4. 测试

部署完成后使用 apache ab 进行测试，如果没有安装的话，使用 **apt install apache2-utils** 安装。

```shell
root@nfs:~# ab -n 1000 -c 10 http://kafka-demo.local.com/submit-order
This is ApacheBench, Version 2.3 <$Revision: 1843412 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking kafka-demo.local.com (be patient)
Completed 100 requests
Completed 200 requests
Completed 300 requests
Completed 400 requests
Completed 500 requests
Completed 600 requests
Completed 700 requests
Completed 800 requests
Completed 900 requests
Completed 1000 requests
Finished 1000 requests


Server Software:
Server Hostname:        kafka-demo.local.com
Server Port:            80

Document Path:          /submit-order
Document Length:        95 bytes

Concurrency Level:      10
Time taken for tests:   84.805 seconds
Complete requests:      1000
Failed requests:        881
   (Connect: 0, Receive: 0, Length: 881, Exceptions: 0)
Total transferred:      212441 bytes
HTML transferred:       95441 bytes
Requests per second:    11.79 [#/sec] (mean)
Time per request:       848.049 [ms] (mean)
Time per request:       84.805 [ms] (mean, across all concurrent requests)
Transfer rate:          2.45 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        0    1   0.2      0       3
Processing:   150  844 5077.7    182   60017
Waiting:      150  844 5077.7    182   60017
Total:        150  844 5077.7    183   60018
ERROR: The median and mean for the initial connection time are more than twice the standard
       deviation apart. These results are NOT reliable.

Percentage of the requests served within a certain time (ms)
  50%    183
  66%    190
  75%    196
  80%    201
  90%    222
  95%    482
  98%   2164
  99%  32078
 100%  60018 (longest request)
```

##### 期望结果

###### Order

- Pod0 接收请求并将订单发送到主题的 0 分区
- Pod0 接收请求并将订单发送到主题的 1 分区
- Pod0 接收请求并将订单发送到主题的 2 分区

###### Repository

- Pod0 消费主题的 0 分区消息
- Pod0 消费主题的 1 分区消息
- Pod0 消费主题的 2 分区消息

###### Statistics

统计服务只有1个Pod，它会消费主题的所有分区消息

##### 结果验证

这个时候我们查看订单服务、仓储服务和统计服务的 Pod 日志，看下是否和我们期望的一致，这里仅列出最后几条日志

##### Order

- Pod0

  ```json
  {"level":"info","time":"2021-12-07T15:36:24+08:00","message":"{\"money\":133,\"order_id\":\"1468122204792164352\",\"time\":1638862584,\"user_id\":\"1468122204460814337\"}"}
  {"level":"info","time":"2021-12-07T15:36:24+08:00","message":"{\"money\":577,\"order_id\":\"1468122204720861184\",\"time\":1638862582,\"user_id\":\"1468122197313720320\"}"}
  {"level":"info","time":"2021-12-07T15:36:24+08:00","message":"{\"money\":133,\"order_id\":\"1468122204905410560\",\"time\":1638862584,\"user_id\":\"1468122204586643456\"}"}
  ```

- Pod1

  ```json
  {"level":"info","time":"2021-12-07T15:37:30+08:00","message":"{\"money\":598,\"order_id\":\"1468122481544925184\",\"time\":1638862650,\"user_id\":\"1468122481209380864\"}"}
  {"level":"info","time":"2021-12-07T15:37:30+08:00","message":"{\"money\":598,\"order_id\":\"1468122481557508096\",\"time\":1638862650,\"user_id\":\"1468122481180020736\"}"}
  {"level":"info","time":"2021-12-07T15:37:30+08:00","message":"{\"money\":598,\"order_id\":\"1468122482039853056\",\"time\":1638862650,\"user_id\":\"1468122481737863168\"}"}
  {"level":"info","time":"2021-12-07T15:37:30+08:00","message":"{\"money\":598,\"order_id\":\"1468122482341842944\",\"time\":1638862650,\"user_id\":\"1468122481976938496\"}"}
  {"level":"info","time":"2021-12-07T15:37:30+08:00","message":"{\"money\":598,\"order_id\":\"1468122482421534720\",\"time\":1638862650,\"user_id\":\"1468122482035658752\"}"}
  ```

- Pod2

  ```json
  {"level":"info","time":"2021-12-07T15:36:26+08:00","message":"{\"money\":878,\"order_id\":\"1468122210429308928\",\"time\":1638862585,\"user_id\":\"1468122210072793088\"}"}
  {"level":"info","time":"2021-12-07T15:36:26+08:00","message":"{\"money\":878,\"order_id\":\"1468122210794213376\",\"time\":1638862586,\"user_id\":\"1468122210471251968\"}"}
  {"level":"info","time":"2021-12-07T15:36:26+08:00","message":"{\"money\":878,\"order_id\":\"1468122210852933632\",\"time\":1638862586,\"user_id\":\"1468122210542555136\"}"}
  {"level":"info","time":"2021-12-07T15:36:26+08:00","message":"{\"money\":336,\"order_id\":\"1468122211255586816\",\"time\":1638862586,\"user_id\":\"1468122210890682368\"}"}
  {"level":"info","time":"2021-12-07T15:36:26+08:00","message":"{\"money\":336,\"order_id\":\"1468122211507245056\",\"time\":1638862586,\"user_id\":\"1468122211201060864\"}"}
  ```

##### Repository

 - Pod0

   ```json
   {"level":"info","time":"2021-12-07T15:37:30+08:00","message":"{\"key\":null,\"offset\":964,\"partition\":1,\"value\":\"{\\\"money\\\":843,\\\"order_id\\\":\\\"1468122479863009280\\\",\\\"time\\\":1638862650,\\\"user_id\\\":\\\"1468122479552630784\\\"}\"}"}
   {"level":"info","time":"2021-12-07T15:37:30+08:00","message":"{\"key\":null,\"offset\":965,\"partition\":1,\"value\":\"{\\\"money\\\":503,\\\"order_id\\\":\\\"1468122479913340928\\\",\\\"time\\\":1638862618,\\\"user_id\\\":\\\"\\\"}\"}"}
   {"level":"info","time":"2021-12-07T15:37:30+08:00","message":"{\"key\":null,\"offset\":966,\"partition\":1,\"value\":\"{\\\"money\\\":552,\\\"order_id\\\":\\\"1468122479934312448\\\",\\\"time\\\":1638862646,\\\"user_id\\\":\\\"1468122477761662976\\\"}\"}"}
   ```
   
 - Pod1

- Pod2

### 5. 总结

License
-------

Under the [MIT](./LICENSE) License