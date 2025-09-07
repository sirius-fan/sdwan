# sdwan (WIP)


一个最小可运行的原型，目标是类似 Tailscale 的 SD-WAN, 主要追求 **易用性** ：
- 控制面：注册/分配 Tunnel IP、生成密钥、返回 peers
- Agent：向控制面注册，周期获取 peers（后续将应用于 WireGuard 配置）
- UDP 中继：占位 Echo 服务，未来提供 NAT 穿透中继

当前状态：
- 当前已经实现基本逻辑,短期先不搞了, 需求动力不足, 我这四五个机器,可以手写路由规则了 😂
- 使用 wgctrl 生成密钥；通过IP命令配置 WireGuard 设备
- 未实现控制面加密传输
- 未实现 NAT 穿透/打洞，仅骨架
- 中继路由未实现，仅 echo


创建控制面节点
<!-- img -->
![Screenshot.png](https://github.com/sirius-fan/sdwan/blob/master/pic/Screenshot_2025-09-07_23%3A25%3A21.png)

节点1
![Screenshot-1.png](https://github.com/sirius-fan/sdwan/blob/master/pic/Screenshot_2025-09-07_23%3A25%3A45.png)

节点2
![Screenshot-2.png](https://github.com/sirius-fan/sdwan/blob/master/pic/Screenshot_2025-09-07_23%3A24%3A07.png)

ping
![Screenshot-3.png](https://github.com/sirius-fan/sdwan/blob/master/pic/Screenshot_2025-09-07_23%3A25%3A00.png)

http server
![Screenshot-4.png](https://github.com/sirius-fan/sdwan/blob/master/pic/Screenshot_2025-09-07_23%3A26%3A26.png)



# sdwan (WIP)

一个最小可运行的 SD-WAN 原型（类似 Tailscale，使用 WireGuard 作为数据面）。

- 控制面：
	- 负责注册、分配 Tunnel IP、生成密钥、维护节点信息（含 Endpoint），下发 peers 列表
	- 接收 Agent 周期上报 Endpoints（动态更新首选 Endpoint）
- Agent：
	- 向控制面注册，自动创建/管理本机 WireGuard 接口（创建接口、配置/刷新 /32 地址、添加路由）
	- 周期拉取 peers 并应用 WireGuard 配置（ReplacePeers）
	- 周期上报 Endpoints 和监听端口，供控制面选择并下发 peer.Endpoint
- UDP 中继：占位 Echo 服务（后续用于 NAT 穿透/转发）

当前限制/范围：
- Endpoint 选择为朴素策略：优先使用 Agent 上报的候选列表中第一个合法 host:port
- peers 的 AllowedIPs 仅包含对端 Tunnel /32；暂未做子网复用/路由聚合
- NAT 打洞逻辑未实现；中继目前只做 echo，占位
- Web/API 未做鉴权/加密，暂用于局域网或实验环境

## 目录结构
- `cmd/controller`：控制器 HTTP 服务
- `cmd/agent`：Agent 客户端
- `cmd/relay`：最简 UDP 中继（回声）
- `internal/common`：公共类型与工具
- `internal/controller`：内存存储与 HTTP Handler
- `internal/agent`：Agent 逻辑与 WireGuard 配置

## 依赖
- Go 1.22+
- Linux 内核支持 WireGuard（建议安装 wireguard-tools 以便调试）
- 需要 root 权限运行 Agent（需执行 `ip` 命令创建/配置接口与路由）
- iproute2（提供 `ip` 命令）

## 构建

```bash
# 编译全部二进制
make build

# 或分别编译
GO111MODULE=on go build -o bin/controller ./cmd/controller
GO111MODULE=on go build -o bin/agent ./cmd/agent
GO111MODULE=on go build -o bin/relay ./cmd/relay
```

## 快速开始

1) 启动控制器（默认 100.64.0.0/16 作为隧道网段）

```bash
./bin/controller -listen :8080 -cidr 100.64.0.0/16
```

2) 在两个不同主机上以 root 启动 Agent（会自动创建并管理 `wg0`，分配 /32 地址并添加路由）：

```bash
sudo ./bin/agent \
	-controller http://<controller-ip>:8080 \
	-hostname node1 \
	-iface wg0 \
	-listen 51820 \
	-endpoint <public-ip-or-lan-ip>:51820   # 可选；不指定则 Agent 会自动收集本机 IPv4 地址组合
```

3) 观察运行：

```bash
# 查看 WireGuard 状态
sudo wg show

# 查看接口与地址/路由
ip addr show dev wg0
ip route show dev wg0

# 尝试从 node1 ping node2 的 Tunnel IP（控制面分配的 100.64.x.x）
ping 100.64.x.x
```

提示：如果你在单机上多开 Agent 仅用于演示，务必使用不同 `-listen` 端口且不要互相使用 127.0.0.1 作为 Endpoint（推荐两个不同容器/虚拟机）。

## 命令行参数

- 控制器 `bin/controller`：
	- `-listen`：HTTP 监听地址（默认 `:8080`）
	- `-cidr`：隧道网段（默认 `100.64.0.0/16`）

- Agent `bin/agent`：
	- `-controller`：控制器基地址（默认 `http://127.0.0.1:8080`）
	- `-hostname`：上报的主机名（默认 `node`）
	- `-iface`：WireGuard 接口名（默认 `wg0`）
	- `-listen`：WireGuard 监听端口（默认 `51820`）
	- `-endpoint`：上报给控制面的首选外部地址（`host:port`，可选）；未指定时 Agent 会自动收集本机 IPv4 地址组合为候选

- 中继 `bin/relay`：
	- `-listen`：UDP 监听地址（默认 `:3478`），当前仅 echo

## API（简要）

- `POST /api/register`（Agent -> Controller）
	- Request：
		- `hostname`、`os`、`version`
		- `endpoints`: `["host:port", ...]`（候选，Agent 可上传 `-endpoint` 或自动收集）
		- `listenPort`: `int`
	- Response：
		- `node`: 节点信息（含 `id`、`tunnelIp`、`publicKey`、`endpoint`）
		- `peers`: 同组其他节点列表（含 `endpoint`）
		- `privKey`: 分配的私钥（Agent 本地使用）
		- `networkCidr`: 隧道网段
		- `relayUdp`: 中继地址（预留，当前为空）

- `GET /api/peers`（Agent -> Controller）
	- Response：`{ peers: [Node...] }`

- `POST /api/announce`（Agent -> Controller）
	- Request：`{ nodeId, endpoints: ["host:port"...], listenPort }`
	- 作用：更新节点候选地址，控制面据此选择并保存 `node.endpoint`

## 行为细节

- Agent 端：
	- 自动创建 `wg0`（若不存在），`ip link add wg0 type wireguard && ip link set up`
	- 为 `wg0` 配置分配的 `/32` 地址（会先清理旧地址以保证幂等），并添加到 `networkCidr` 的设备路由
	- peers 配置采用 ReplacePeers；每个 peer 的 AllowedIPs 为对端 `/32`，若提供 `endpoint` 则设为 `PeerConfig.Endpoint`
	- 每 15s 拉取 peers 并应用；同时上报 `announce`（自动收集本机 IPv4 地址拼 `listenPort` 作为候选）

- 控制面：
	- `register` 时分配 `tunnelIp`、生成密钥、保存节点信息
	- `announce` 时更新节点 `endpoints` 并重选 `endpoint`（当前策略：候选中第一个合法 `host:port`）

## 常见问题（Troubleshooting）


## 路线图（Roadmap）

- Endpoint 选择策略：结合 `Announce` 的来源地址和探测结果做优选/多地址回退
- NAT 打洞（hole punching）与中继转发通道
- 控制面鉴权、Agent 认证与心跳/过期清理
- 配置持久化、多副本控制器
- 使用 netlink 替代外部 `ip` 命令，提升健壮性
- 更丰富的路由/子网宣告与 ACL

---

仅供学习与实验，请勿用于生产环境。
