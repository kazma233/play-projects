export const appMeta = {
  label: 'Kt 工具箱',
  icon: '🧰',
  description: '常用开发工具集合'
}

export const toolItems = [
  {
    label: '时间转换',
    path: '/datetime',
    icon: '⏰',
    eyebrow: '时间工具',
    description: '统一处理时间戳、RFC3339 与时区换算。',
    highlights: ['时间戳', '时区', '日期计算']
  },
  {
    label: 'Base64',
    path: '/base64',
    icon: '🧾',
    eyebrow: '编码工具',
    description: '支持 Base64 编解码与 URL 安全模式转换。',
    highlights: ['实时转换', 'URL Safe', '双向编辑']
  },
  {
    label: 'URL 编码',
    path: '/url',
    icon: '🔗',
    eyebrow: '请求工具',
    description: '用于处理链接、查询参数与接口中的编码字段。',
    highlights: ['查询串', 'URL Decode', '实时转换']
  },
  {
    label: 'JSON 格式化',
    path: '/json',
    icon: '🧩',
    eyebrow: '数据工具',
    description: '支持 JSON 格式化、树形浏览与 JsonPath 过滤。',
    highlights: ['Tree View', 'JsonPath', '格式校验']
  },
  {
    label: 'MD5 计算',
    path: '/md5',
    icon: '🔐',
    eyebrow: '哈希工具',
    description: '用于计算文本 MD5。',
    highlights: ['文本哈希', '结果复制', '单击计算']
  },
  {
    label: 'SHA1 计算',
    path: '/sha1',
    icon: '🪪',
    eyebrow: '文件校验',
    description: '用于生成本地文件的 SHA1 校验值。',
    highlights: ['文件选择', '校验值', '结果复制']
  },
  {
    label: '二维码',
    path: '/qrcode',
    icon: '📱',
    eyebrow: '分享工具',
    description: '支持二维码生成、自定义颜色尺寸、保存和复制。',
    highlights: ['颜色控制', 'PNG 导出', '剪贴板']
  },
  {
    label: '端口管理',
    path: '/ports',
    icon: '🔌',
    eyebrow: '系统诊断',
    description: '查看本机端口占用、协议、地址与进程信息。',
    highlights: ['端口扫描', '筛选检索', '快速终止']
  }
]

export const toolMetaMap = Object.fromEntries(toolItems.map((item) => [item.path, item]))
