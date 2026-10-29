# DIA／Dominate 原文与实现映射

DIS-reconstruct 依据 DIA／Dominate 描述的架构与业务算法，重建身份、数据标识和授权管理，并接通 DNS 解析。

## 组件映射

| 原文组件或能力 | 重建组件 | 业务目的 |
|---|---|---|
| Identity Management | 身份合约和公钥查询 | 登记身份，支持请求者认证 |
| Identifier Management | 数据登记和属性查询 | 将数据绑定到可解析的标识 |
| Authorization Management | 授权申请与所有者决定 | 维护申请、批准及拒绝状态 |
| Blockchain Platform | Fabric 网络与管理合约 | 按共同规则执行并保存更新 |
| Identifier System | 同步服务与 CoreDNS | 对外解析标识及授权记录 |
| Dominate 的 DIS-hub 接入方式 | DIS API | 提供统一业务入口 |

## 业务语义

应用申请产生 pending 授权记录；所有者作出批准或拒绝决定，将记录更新为 approved 或 rejected。Fabric 背书和提交记录这些业务操作。

数据注册绑定所有者、地址、数据签名与描述。应用取得标识后查询属性，并使用授权流程表达访问请求。

## 重建依据

DIA 对应第三节的系统组件与交互流程、Algorithm 1—3，以及第四节的实现。Dominate 对应系统设计与第八节的实现及评价。

本项目实现两篇论文中的标识注册、管理与解析路径。DIA 的独立身份系统、Dominate 的数据发现服务与完整数据存储服务，通过相应查询和数据地址与这一路径衔接。

具体编码、确定性标识生成和时间字段的工程选择记录在[设计决策](decisions.md)，组件接通情况见[运行手册](runbook.md)。
