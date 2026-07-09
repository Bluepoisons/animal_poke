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

  // 导航
  'tab.camera': '发现',
  'tab.collection': '图鉴',
  'tab.fight': '战斗',
  'tab.store': '商店',
  'tab.dispatch': '派遣',
  'tab.achievement': '成就',

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

  // 捕获
  'capture.throw': '投掷',
  'capture.success': '捕获成功！',
  'capture.fail': '差一点！再试一次',
  'capture.power': '力度',

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

  // 设置
  'settings.language': '语言',
  'settings.chinese': '中文',
  'settings.english': 'English',
} as const

export type TranslationKey = keyof typeof zh
