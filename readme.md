Kafka Demo
=====

### 1. 概述

这个项目是 Go 实现的 Kafka Demo，主要功能是模拟商城的下单流程，其中下单后订单服务会向 Kafka 生成一条信息， 仓储服务会从 Kafka 消费消息进行打包发货等处理，统计服务会从 Kafka 消费消息进行订单的统计。这个 Demo 的重点在于 Kafka 消息的生产和消费，因此各个服务端的代码都省略，仅打印相关的日志信息用于校验数据。

这次测试的所有源码都托管在 [Github](https://github.com/jormin/kafka-demo)。

#### 1.1 模拟商城

这里我们模拟的是一个简单的商城流程，假设商城服务省份为陕西省，有三个区域，分别是 陕北、关中、陕南，分别对应的分区编号是 0，1，2，这三个区域的下单及打包发货流程是独立的，但是商城的统计是整体的，我们期望的是下单时候对应分区的订单提交到对应分区的订单服务，并由对应分区的仓储服务进行打包发货，最后由统计服务统计所有分区的订单信息，如下：

| 分区 | 编号 | 订单服务 | 仓储服务     | 统计服务       |
| ---- | ---- | -------- | ------------ | -------------- |
| 陕北 | 0    | Pod-0    | repository-0 | statistics-xxx |
| 关中 | 1    | Pod-1    | repository-1 | statistics-xxx |
| 陕南 | 2    | Pod-2    | repository-2 | statistics-xxx |

#### 1.2 项目流程

1. 用户端调用 Api 网关层的下单接口。
2. 网关层生成随机用户ID及订单金额，**并根据用户请求参数中的分区编号**，将订单提交给**对应编号的订单服务**，订单服务返回生成的订单信息，网关层记录日志并给用户发送响应信息。
3. 订单服务接收到网关层请求后生成订单信息并将订单信息写入 Kafka，写入的主题分区从 hostname 中获取，之后记录日志。
4. 仓储服务从 Kafka 消费消息，消费的主题分区从 hostname 获取，之后记录日志。
5. 统计服务从 Kafka 消费改主题下的所有分区的消息，之后记录日志。

流程图如下：

![1](https://blog.cdn.lerzen.com/img/20211206155214.png)

### 2. 资源清单

#### 2.1 K8s集群

| 节点   | 角色         | IP            | 配置      | Label               |
| ------ | ------------ | ------------- | --------- | ------------------- |
| master | master, etcd | 192.168.1.100 | 4核4G50G  | usefulness=schedule |
| node1  | worker       | 192.168.1.101 | 8核8G100G | usefulness=devops   |
| node2  | worker       | 192.168.1.102 | 8核8G100G | usefulness=demo     |
| node3  | worker       | 192.168.1.103 | 8核8G100G | usefulness=business |

#### 2.2 部署方案

本次部署的所有资源都在命名空间 **kafka-demo** 下，所有 Pod 都部署在 **node3** 节点上。

1. Api 网关层 - gateway：部署为 Deployment，启动 10 个 Pod
2. 订单服务 - order：部署为 Statefulset，启动 3 个 Pod
3. 仓储服务 - repository：部署为  Statefulset，启动 3 个 Pod
4. 统计服务 - statistics：部署为  Deployment，启动 1 个 Pod
5. Kafka：使用 [helm安装zookeeper和kafka](https://blog.lerzen.com/post/helm%E5%AE%89%E8%A3%85zookeeper%E5%92%8Ckafka/) 中部署好的 Kafka，使用的 Topic 为 **order**，有三个分区，两个备份

#### 2.3 域名

| 域名                          | 类型         | IP            | 资源 |
| ----------------------------- | ------------ | ------------- | ------- |
| kafka-demo.local.com                | IngressRoute | 192.168.1.100 | gateway service |
| order.kafka-demo.svc.cluster.local | Service DNS | -----         | order service |
| order-0.order.kafka-demo.svc.cluster.local | Pod DNS | ----- | order pod-0 |
| order-1.order.kafka-demo.svc.cluster.local | Pod DNS | ----- | order pod-1 |
| order-2.order.kafka-demo.svc.cluster.local | Pod DNS | ----- | order pod-2 |

#### 2.4 镜像

打包的镜像我是放在自己的 harbor 上，也可以托管在 docker hub、腾讯云 或者 阿里云等等，对应的需要修改 **build.sh** 和 **k8s/deploy/*.yaml** 中的镜像地址、登录信息以及拉取镜像的秘钥配置等。

### 3. 部署

#### 3.1 域名写入 hosts 信息

由于我的集群中，traefik 部署在 node1(192.168.1.101) 上，因此需要将网关的域名解析到 node1 上。

```shell
cat >> /etc/hosts <<EOF
192.168.1.101 kafka-demo.local.com
EOF
```

#### 3.2 创建 Topic

部署服务之前，**需要先创建一个名为 **order** 的 Topic**，这里我们通过 kafka client pod 来创建。

- 创建并连接 kafka client pod

  ```shell
  root@master:~/kafka-demo/k8s/deploy# kubectl run kafka-client --restart='Never' --image docker.io/bitnami/kafka:2.8.1-debian-10-r57 --namespace devops --command -- sleep infinity
  pod/kafka-client created
  
  root@master:~/kafka-demo/k8s/deploy# kubectl exec --tty -i kafka-client --namespace devops -- bash
  I have no name!@kafka-client:/$
  ```

- 连接上 kafka client pod 后，管理 Topic

  ```shell
  # 查看 Topic 列表
  I have no name!@kafka-client:/$ kafka-topics.sh --bootstrap-server kafka.devops.svc.cluster.local:9092 --list
  __consumer_offsets
  
  # 创建 Topic
  I have no name!@kafka-client:/$ kafka-topics.sh --bootstrap-server kafka.devops.svc.cluster.local:9092 --create --topic order --partitions 3 --replication-factor 2
  Created topic order.
  
  # 删除 Topic
  I have no name!@kafka-client:/$ kafka-topics.sh --bootstrap-server kafka.devops.svc.cluster.local:9092 --delete --topic order
  ```

#### 3.3 创建命名空间

**scheduler.alpha.kubernetes.io/node-selector: usefulness=business** 是为了让该命名空间下的所有 Pod 都部署在有标签 **usefulness=business**  的节点上，也就是 node3(192.168.1.103)。

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: kafka-demo
  annotations:
    scheduler.alpha.kubernetes.io/node-selector: usefulness=business
```

#### 3.4 部署网关层

Gateway 部署为 Deployment，启动10个Pod，需要配一个 Service 和 IngressRoute，使用域名 **kafka-demo.local.com** 访问，资源清单如下：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
  namespace: kafka-demo
spec:
  replicas: 10
  minReadySeconds: 2
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
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
          readinessProbe:
            periodSeconds: 1
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

#### 3.5 部署订单服务

Order 部署为 StatefulSet，启动3个Pod，需要配一个 Service ，使用内部域名 **order.kafka-demo.svc.clauster.local** 访问，资源清单如下：

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: order
  namespace: kafka-demo
spec:
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
          readinessProbe:
            periodSeconds: 1
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

#### 3.6 部署仓储服务

Repository 部署为 StatefulSet，启动3个Pod，资源清单如下：

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: repository
  namespace: kafka-demo
spec:
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
          readinessProbe:
            periodSeconds: 1
            initialDelaySeconds: 1
            exec:
              command:
                - ls
```

#### 3.7 部署统计服务

Statistics 服务部署为 Deployment，只需要启动1个Pod，资源清单如下：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: statistics
  namespace: kafka-demo
spec:
  replicas: 1
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
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
          readinessProbe:
            periodSeconds: 1
            initialDelaySeconds: 1
            exec:
              command:
                - ls
```

以上清单部署使用 **kubectl apply -f xxx.yaml** 即可。

#### 3.8  部署结果

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

这里我们使用 curl 调用网关的下单接口，请求中指定分区信息，记录网关响应的订单信息，结合订单、仓储和统计服务的日志信息，追踪订单的服务链路，如果一致即可认为测试通过。

Curl 请求脚本：

```shell
#!/bin/bash

# 分区0下单
curl http://kafka-demo.local.com/submit-order -d "partition=0" && echo ""
curl http://kafka-demo.local.com/submit-order -d "partition=0" && echo ""
curl http://kafka-demo.local.com/submit-order -d "partition=0" && echo ""

# 分区1下单
curl http://kafka-demo.local.com/submit-order -d "partition=1" && echo ""
curl http://kafka-demo.local.com/submit-order -d "partition=1" && echo ""
curl http://kafka-demo.local.com/submit-order -d "partition=1" && echo ""

# 分区2下单
curl http://kafka-demo.local.com/submit-order -d "partition=2" && echo ""
curl http://kafka-demo.local.com/submit-order -d "partition=2" && echo ""
curl http://kafka-demo.local.com/submit-order -d "partition=2" && echo ""
```

请求结果：

```shell
root@nfs:~# bash test.sh
{"money":81,"order_id":"1468136426238382080","time":1638865975,"user_id":"1468136425655373824"}
{"money":81,"order_id":"1468136427404398592","time":1638865975,"user_id":"1468136427060465664"}
{"money":81,"order_id":"1468136428645912576","time":1638865975,"user_id":"1468136428268425216"}
{"money":81,"order_id":"1468136430059393024","time":1638865975,"user_id":"1468136429430247424"}
{"money":81,"order_id":"1468136431246381056","time":1638865976,"user_id":"1468136430894059520"}
{"money":81,"order_id":"1468136432269791232","time":1638865976,"user_id":"1468136431942635520"}
{"money":81,"order_id":"1468136433578414080","time":1638865976,"user_id":"1468136432978628608"}
{"money":81,"order_id":"1468136434660544512","time":1638865977,"user_id":"1468136434329194496"}
{"money":81,"order_id":"1468136435734286336","time":1638865977,"user_id":"1468136435390353408"}
```

#### 4.1 期望结果

##### Order

- Pod0 接收请求并将订单发送到主题的 0 分区
- Pod1 接收请求并将订单发送到主题的 1 分区
- Pod2 接收请求并将订单发送到主题的 2 分区

##### Repository

- Pod0 消费主题的 0 分区消息
- Pod1 消费主题的 1 分区消息
- Pod2 消费主题的 2 分区消息

##### Statistics

统计服务只有1个Pod，它会消费主题的所有分区消息

#### 4.2 日志收集

##### Order

- Pod0

  ```shell
  root@master:~/kafka-demo/k8s/deploy# kubectl logs order-0 -n kafka-demo
  ......
  {"level":"info","time":"2021-12-07T16:32:55+08:00","message":"{\"money\":81,\"order_id\":\"1468136426238382080\",\"time\":1638865975,\"user_id\":\"1468136425655373824\"}"}
  {"level":"info","time":"2021-12-07T16:32:55+08:00","message":"{\"money\":81,\"order_id\":\"1468136427404398592\",\"time\":1638865975,\"user_id\":\"1468136427060465664\"}"}
  {"level":"info","time":"2021-12-07T16:32:55+08:00","message":"{\"money\":81,\"order_id\":\"1468136428645912576\",\"time\":1638865975,\"user_id\":\"1468136428268425216\"}"}
  ```

- Pod1

  ```shell
  root@master:~/kafka-demo/k8s/deploy# kubectl logs order-1 -n kafka-demo
  ......
  {"level":"info","time":"2021-12-07T16:32:56+08:00","message":"{\"money\":81,\"order_id\":\"1468136430059393024\",\"time\":1638865975,\"user_id\":\"1468136429430247424\"}"}
  {"level":"info","time":"2021-12-07T16:32:56+08:00","message":"{\"money\":81,\"order_id\":\"1468136431246381056\",\"time\":1638865976,\"user_id\":\"1468136430894059520\"}"}
  {"level":"info","time":"2021-12-07T16:32:56+08:00","message":"{\"money\":81,\"order_id\":\"1468136432269791232\",\"time\":1638865976,\"user_id\":\"1468136431942635520\"}"}
  ```

- Pod2

  ```shell
  root@master:~/kafka-demo/k8s/deploy# kubectl logs order-2 -n kafka-demo
  ......
  {"level":"info","time":"2021-12-07T16:32:57+08:00","message":"{\"money\":81,\"order_id\":\"1468136433578414080\",\"time\":1638865976,\"user_id\":\"1468136432978628608\"}"}
  {"level":"info","time":"2021-12-07T16:32:57+08:00","message":"{\"money\":81,\"order_id\":\"1468136434660544512\",\"time\":1638865977,\"user_id\":\"1468136434329194496\"}"}
  {"level":"info","time":"2021-12-07T16:32:57+08:00","message":"{\"money\":81,\"order_id\":\"1468136435734286336\",\"time\":1638865977,\"user_id\":\"1468136435390353408\"}"}
  ```

##### Repository

 - Pod0

   ```shell
   root@master:~/kafka-demo/k8s/deploy# kubectl logs repository-0 -n kafka-demo
   {"level":"info","time":"2021-12-07T16:32:55+08:00","message":"{\"key\":null,\"offset\":0,\"partition\":0,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136426238382080\\\",\\\"time\\\":1638865975,\\\"user_id\\\":\\\"1468136425655373824\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:55+08:00","message":"{\"key\":null,\"offset\":1,\"partition\":0,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136427404398592\\\",\\\"time\\\":1638865975,\\\"user_id\\\":\\\"1468136427060465664\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:55+08:00","message":"{\"key\":null,\"offset\":2,\"partition\":0,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136428645912576\\\",\\\"time\\\":1638865975,\\\"user_id\\\":\\\"1468136428268425216\\\"}\"}"}
   ```
   
 - Pod1

   ```shell
   root@master:~/kafka-demo/k8s/deploy# kubectl logs repository-1 -n kafka-demo
   {"level":"info","time":"2021-12-07T16:32:56+08:00","message":"{\"key\":null,\"offset\":0,\"partition\":1,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136430059393024\\\",\\\"time\\\":1638865975,\\\"user_id\\\":\\\"1468136429430247424\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:56+08:00","message":"{\"key\":null,\"offset\":1,\"partition\":1,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136431246381056\\\",\\\"time\\\":1638865976,\\\"user_id\\\":\\\"1468136430894059520\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:56+08:00","message":"{\"key\":null,\"offset\":2,\"partition\":1,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136432269791232\\\",\\\"time\\\":1638865976,\\\"user_id\\\":\\\"1468136431942635520\\\"}\"}"}
   ```

- Pod2

  ```shell
  root@master:~/kafka-demo/k8s/deploy# kubectl logs repository-2 -n kafka-demo
  {"level":"info","time":"2021-12-07T16:32:57+08:00","message":"{\"key\":null,\"offset\":0,\"partition\":2,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136433578414080\\\",\\\"time\\\":1638865976,\\\"user_id\\\":\\\"1468136432978628608\\\"}\"}"}
  {"level":"info","time":"2021-12-07T16:32:57+08:00","message":"{\"key\":null,\"offset\":1,\"partition\":2,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136434660544512\\\",\\\"time\\\":1638865977,\\\"user_id\\\":\\\"1468136434329194496\\\"}\"}"}
  {"level":"info","time":"2021-12-07T16:32:57+08:00","message":"{\"key\":null,\"offset\":2,\"partition\":2,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136435734286336\\\",\\\"time\\\":1638865977,\\\"user_id\\\":\\\"1468136435390353408\\\"}\"}"}
  ```

##### Statistics

 - Pod0

   ```shell
   root@master:~/kafka-demo/k8s/deploy# kubectl logs statistics-5fb5557d68-xnz7z -n kafka-demo
   {"level":"info","time":"2021-12-07T16:32:55+08:00","message":"{\"key\":null,\"offset\":0,\"partition\":0,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136426238382080\\\",\\\"time\\\":1638865975,\\\"user_id\\\":\\\"1468136425655373824\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:55+08:00","message":"{\"key\":null,\"offset\":1,\"partition\":0,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136427404398592\\\",\\\"time\\\":1638865975,\\\"user_id\\\":\\\"1468136427060465664\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:55+08:00","message":"{\"key\":null,\"offset\":2,\"partition\":0,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136428645912576\\\",\\\"time\\\":1638865975,\\\"user_id\\\":\\\"1468136428268425216\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:56+08:00","message":"{\"key\":null,\"offset\":0,\"partition\":1,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136430059393024\\\",\\\"time\\\":1638865975,\\\"user_id\\\":\\\"1468136429430247424\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:56+08:00","message":"{\"key\":null,\"offset\":1,\"partition\":1,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136431246381056\\\",\\\"time\\\":1638865976,\\\"user_id\\\":\\\"1468136430894059520\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:56+08:00","message":"{\"key\":null,\"offset\":2,\"partition\":1,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136432269791232\\\",\\\"time\\\":1638865976,\\\"user_id\\\":\\\"1468136431942635520\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:57+08:00","message":"{\"key\":null,\"offset\":0,\"partition\":2,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136433578414080\\\",\\\"time\\\":1638865976,\\\"user_id\\\":\\\"1468136432978628608\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:57+08:00","message":"{\"key\":null,\"offset\":1,\"partition\":2,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136434660544512\\\",\\\"time\\\":1638865977,\\\"user_id\\\":\\\"1468136434329194496\\\"}\"}"}
   {"level":"info","time":"2021-12-07T16:32:57+08:00","message":"{\"key\":null,\"offset\":2,\"partition\":2,\"value\":\"{\\\"money\\\":81,\\\"order_id\\\":\\\"1468136435734286336\\\",\\\"time\\\":1638865977,\\\"user_id\\\":\\\"1468136435390353408\\\"}\"}"}
   ```

#### 4.3 结果验证

本次测试我们一共下了9个订单，期望的服务Podh和测试结果如下，测试通过。

| 订单号              | 期望订单 | 期望仓储 | 期望统计 | Topic分区 | 测试结果 |
| ------------------- | -------- | -------- | -------- | --------- | -------- |
| 1468136426238382080 | Pod0     | Pod0     | Pod0     | 0         | ✅        |
| 1468136427404398592 | Pod0     | Pod0     | Pod0     | 0         | ✅        |
| 1468136428645912576 | Pod0     | Pod0     | Pod0     | 0         | ✅        |
| 1468136430059393024 | Pod1     | Pod1     | Pod0     | 1         | ✅        |
| 1468136431246381056 | Pod1     | Pod1     | Pod0     | 1         | ✅        |
| 1468136432269791232 | Pod1     | Pod1     | Pod0     | 1         | ✅        |
| 1468136433578414080 | Pod2     | Pod2     | Pod0     | 2         | ✅        |
| 1468136434660544512 | Pod2     | Pod2     | Pod0     | 2         | ✅        |
| 1468136435734286336 | Pod2     | Pod2     | Pod0     | 2         | ✅        |

### 5. 总结

本篇文章主要是为了验证 Topic 分区的订阅和消费机制，通过模拟商城的下单流程，通过让对应的分区订单让对应的分区订单和仓储服务处理来验证单个分区消息的生产和消费，通过让统计服务统计所有订单来模拟消费所有分区消息，最终测试结果和预期一致。

License
-------

Under the [MIT](./LICENSE) License