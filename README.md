# api-compare

Compare the apis of venus and lotus


## Start

### build

```sh
make
```

### run

```sh
./apicompare --venus-url=<venus url> --venus-token=<venus token> --lotus-url=<lotus url> --lotus-token=<lotus token>
```

### 对比 ETH 接口

由于节点默认是不开启 `ETH` 接口的访问，如果需要测试 `ETH` 相关接口，需要调整节点的配置

```
$ vi ~/.venus/config.json
# 把 fevm.EnableEthRPC 改为true
"fevm": {
    "EnableEthRPC": true,
}

$ vi ~/.lotus/config.toml
# 把 Fevm.EnableEthRPC 改为true
[Fevm]
  EnableEthRPC = true 
```
