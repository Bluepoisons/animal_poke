/** 中英文案 */
export const zh = {
  // 通用
  'app.name': '动物捕捉',
  'common.confirm': '确认',
  'common.cancel': '取消',
  'common.retry': '重试',
  'common.loading': '加载中…',
  'common.close': '关闭',
  'common.save': '保存',
  'common.back': '返回',
  'common.on': '开',
  'common.off': '关',
  'common.export': '导出',
  'common.delete': '删除',

  // 导航
  'tab.camera': '发现',
  'tab.collection': '图鉴',
  'tab.fight': '战斗',
  'tab.store': '商店',
  'tab.dispatch': '派遣',
  'tab.achievement': '成就',
  'tab.settings': '设置',
  'tab.map': '地图',

  // 体力
  'stamina.label': '体力',
  'stamina.insufficient': '⚡ 体力不足',
  'stamina.full': '体力已满',

  // 发现
  'discover.scanning': '扫描中…',
  'discover.detected': '发现动物！开始捕获',
  'discover.notfound': '未发现动物',
  'discover.error': '检测失败，请重试',
  'discover.cameraDenied': '摄像头权限被拒绝',
  'discover.cameraPrompt': '请允许摄像头权限以发现动物',
  'discover.title': '发现',
  'discover.subtitle': '用镜头寻找身边的小伙伴',

  // 捕获
  'capture.throw': '投掷',
  'capture.success': '捕获成功！',
  'capture.fail': '差一点！再试一次',
  'capture.power': '力度',
  'capture.title': '捕获',
  'capture.stopFirst': '请先停下再操作，避免边走边看屏幕',

  // 图鉴
  'collection.title': '图鉴',
  'collection.empty': '还没有收藏的动物',
  'collection.unlocked': '已收集',
  'collection.total': '总数',
  'collection.filter.all': '全部',
  'collection.filter.today': '今日',
  'collection.filter.week': '本周',
  'collection.filter.nearby': '附近',

  // 战斗
  'battle.title': '战斗',
  'battle.win': '胜利！',
  'battle.lose': '失败…',
  'battle.attack': '攻击',
  'battle.selectPet': '选择宠物',

  // 商店
  'store.title': '商店',
  'store.checkIn': '每日签到',
  'store.checkInClaimed': '今日已签到',
  'store.gold': '金币',
  'store.buy': '购买',

  // 成就
  'achievement.title': '成就',
  'achievement.locked': '未解锁',

  // 错误
  'error.title': '出了点问题',
  'error.crash': '应用崩溃了',
  'error.reload': '重新加载',
  'error.provider': '模块暂时不可用',

  // 设置中心
  'settings.title': '设置',
  'settings.subtitle': '语言、声音、权限与数据',
  'settings.language': '语言',
  'settings.chinese': '中文',
  'settings.english': 'English',
  'settings.japanese': '日本語（预留）',
  'settings.section.audio': '声音',
  'settings.section.motion': '动效与触觉',
  'settings.section.data': '流量与数据',
  'settings.section.privacy': '权限与隐私',
  'settings.sfx': '音效',
  'settings.music': '音乐',
  'settings.haptics': '触觉反馈',
  'settings.motion': '减少动效',
  'settings.dataSaver': '节省流量',
  'settings.permissions': '系统权限说明',
  'settings.permissions.desc': '相机用于发现与捕获；定位用于附近发现点。可在系统设置中管理。',
  'settings.export': '导出本地数据',
  'settings.export.done': '已导出到剪贴板/下载',
  'settings.delete': '删除本地数据',
  'settings.delete.confirm': '确定删除本地设置与缓存？此操作不可撤销。',
  'settings.delete.done': '本地数据已清除',
  'settings.sync.hint': '账号绑定后可安全同步非敏感偏好（语言、音效等）',
  'settings.saved': '已保存',
  'map.title': '猎取地图',
  'map.subtitle': '附近发现点 · 手绘地图',
  'map.you': '你的位置',
  'map.back': '返回手账',
} as const

export type TranslationKey = keyof typeof zh
