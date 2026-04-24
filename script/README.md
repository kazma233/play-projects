# Scripts

## make-rounded-icon.swift

将图片处理成带透明留边的圆角 PNG 图标，非方图会自动居中裁切。

### 特性

1. 非方图自动居中裁切为 `1:1`
2. 输出透明背景 PNG
3. 支持透明留边配置
4. 支持圆角半径配置
5. 支持外阴影开关
6. 支持阴影透明度、模糊半径、偏移配置
7. 提供 `macOS`/`Linux` 两份平台专用脚本

### 使用方法

自动分发入口：

```bash
swift script/make-rounded-icon.swift <input> <output> [options]
```

平台专用入口：

```bash
swift script/make-rounded-icon-macos.swift <input> <output> [options]
swift script/make-rounded-icon-linux.swift <input> <output> [options]
```

默认生成一个 `1024x1024` 的透明 PNG，原图会被裁成圆角并保留一圈透明边距。产物构建、CI 或平台脚本里，建议直接调用对应平台的专用脚本。

### 示例

默认参数：

```bash
swift script/make-rounded-icon.swift input.png output.png
```

关闭阴影并调整留边和圆角：

```bash
swift script/make-rounded-icon-macos.swift input.png output.png \
  --inset 120 \
  --corner-radius 180 \
  --shadow off
```

自定义完整参数：

```bash
swift script/make-rounded-icon-linux.swift input.png output.png \
  --canvas 1024 \
  --inset 96 \
  --corner-radius 220 \
  --shadow on \
  --shadow-alpha 0.22 \
  --shadow-blur 32 \
  --shadow-offset-x 0 \
  --shadow-offset-y -10
```

### 参数

1. `--canvas <size>`
   输出画布尺寸，默认 `1024`
2. `--inset <size>`
   图标四周的透明留边，默认 `96`
3. `--corner-radius <size>`
   圆角半径，默认 `220`
4. `--shadow <on|off>`
   是否开启外阴影，默认 `on`
5. `--shadow-alpha <value>`
   阴影透明度，取值范围 `0` 到 `1`，默认 `0.22`
6. `--shadow-blur <size>`
   阴影模糊半径，默认 `32`
7. `--shadow-offset-x <size>`
   阴影横向偏移，默认 `0`
8. `--shadow-offset-y <size>`
   阴影纵向偏移，默认 `-10`

### 限制

1. 输入图片会先被裁成正方形，默认使用中心区域
2. 输出始终为正方形 PNG
3. `make-rounded-icon-macos.swift` 仅支持 macOS
4. `make-rounded-icon-linux.swift` 仅支持 Linux，且要求系统已安装 `ImageMagick`
5. 如果 `--inset` 过大，或者 `--corner-radius` 超过内部图形尺寸上限，脚本会报错退出

### 常见报错

原图无法读取：

```text
Unable to load source image: /path/to/input.png
```
