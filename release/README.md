# 发布与更新

更新清单固定托管在 GitHub 公开仓库：

- https://github.com/Tangerine-Living/indie-studio
- 稳定版：`stable/latest.json` + `stable/latest.json.sig`
- 开发版：`dev/latest.json` + `dev/latest.json.sig`

应用内不提供清单地址设置，更新源由产品固定。`release/public.b64` 是随源码提交的
Ed25519 公钥，构建时通过 `//go:embed` 嵌入应用；私钥不在这里，也不应进入任何仓库。

## 发布一次更新

1. 构建安装包：`wails build -nsis`
2. 设置 `GH_TOKEN`（有该仓库写权限的 GitHub token，只在当前终端使用）
3. 运行发布脚本：

```powershell
$env:GH_TOKEN = "<token>"
.\scripts\publish-update.ps1 -Version 1.0.0 -Channel stable `
  -Installer build\bin\pixelbench-amd64-installer.exe -Notes "首个稳定版"
```

开发版用 `-Channel dev`。脚本会创建 Release、上传安装包、用私钥生成签名清单，
再把 `latest.json` 与 `.sig` 提交到对应通道目录。

私钥默认在 `..\pixelbench-release-keys\private.b64`（与项目目录同级），也可以用
`-KeyFile` 指定。请离线备份，丢失后旧版本将无法验证新清单。