# go-stock

基于大语言模型的 AI 赋能股票分析桌面工具（Wails + Vue + Go）。

![go-stock](./build/appicon.png)

## 简介

- 支持 A 股、港股、美股行情查看与自选股监控
- 市场资讯聚合、技术指标分析、AI 辅助研究（仅供学习研究，不构成投资建议）
- 数据默认保存在本机；开发环境以 Windows 为主，其他平台功能可能受限

## 使用手册

详见 [go-stock 使用手册](docs/go-stock使用手册.md) 与 [功能文档](docs/功能文档.md)。

## 构建

```powershell
Set-Location D:\stock
wails build
```

产物：`build\bin\go-stock.exe`

仅改前端时可先 `cd frontend; npm run build`，再 `wails build -skipbindings -s`。

## 支持的大模型平台

| 平台 | 说明 |
| --- | --- |
| OpenAI 兼容接口 | 可接入任意 OpenAI 格式 API |
| Ollama / LM Studio | 本地大模型 |
| DeepSeek | deepseek-chat / deepseek-reasoner |
| 硅基流动、火山方舟等 | 需在设置中自行配置 API Key |

## 技术支持

- 邮箱：support@go-stock.app
- 安全问题请参阅 [SECURITY.md](SECURITY.md)

## 免责声明

本软件及 AI 分析结果仅供学习研究。投资有风险，决策请自行判断。

## License

[GNU GPLv3](LICENSE)
