import type { PresentationSequence } from './types'

/** Demo sequence for AP-122 runtime (data-driven; not hard-coded in screens). */
export const sampleRiverNightSequence: PresentationSequence = {
  id: 'demo.river-night.intro',
  title: '沿河夜话 · 序',
  version: '1',
  entryId: 's1',
  segments: [
    {
      id: 's1',
      kind: 'comic_v',
      maxSeconds: 40,
      autoMs: 3000,
      summary: '四格竖屏：河堤路灯亮起，你推开调查本。',
      comic: {
        panels: [
          { id: 'p1', caption: '河风带着潮气', alt: '河堤夜景示意', imageUrl: '/content/demo/panel1.webp' },
          { id: 'p2', caption: '路灯一盏盏点亮', alt: '路灯示意', imageUrl: '/content/demo/panel2.webp' },
          { id: 'p3', caption: '本子翻开第一页', alt: '调查本示意' },
          { id: 'p4', caption: '今晚，从这里开始', alt: '夜色收尾' },
        ],
      },
    },
    {
      id: 's2',
      kind: 'dialogue',
      summary: '向导提醒：注意脚下与时间。',
      dialogue: {
        lines: [
          { id: 'd1', speaker: '向导', text: '滨水步道晚上人少，记得结伴。' },
          { id: 'd2', speaker: '你', text: '我先记录灯光与声响。' },
        ],
      },
    },
    {
      id: 's3',
      kind: 'postcard',
      summary: '明信片：滨水观景台坐标已标记（粗粒度）。',
      postcard: {
        title: '观景台夜色',
        placeLabel: '滨水区 · 观景台',
        body: '潮位牌旁有一只常出没的夜行小兽线索。',
        imageUrl: '/content/demo/postcard.webp',
      },
    },
    {
      id: 's4',
      kind: 'voice_note',
      summary: '语音便签：远处传来低沉水声。',
      voiceNote: {
        audioUrl: '/content/demo/note.mp3',
        transcript: '（低声）水流变急了……也许是闸门在调。',
        speaker: '未知',
      },
    },
    {
      id: 's5',
      kind: 'static_image',
      summary: '关键选择：先跟灯光还是先听水声。',
      staticImage: {
        caption: '岔路口',
        alt: '步道分叉示意',
        imageUrl: '/content/demo/fork.webp',
      },
      choice: {
        prompt: '你接下来？',
        options: [
          { id: 'c_light', label: '跟着路灯走', next: 's6a' },
          { id: 'c_water', label: '靠近水声', next: 's6b' },
        ],
      },
    },
    {
      id: 's6a',
      kind: 'dialogue',
      summary: '选择路灯：发现海报上的活动贴纸。',
      dialogue: {
        lines: [{ id: 'd3', speaker: '你', text: '贴纸上写着「沿河不眠」。' }],
      },
    },
    {
      id: 's6b',
      kind: 'dialogue',
      summary: '选择水声：潮位牌刻痕比昨天更深。',
      dialogue: {
        lines: [{ id: 'd4', speaker: '你', text: '潮位……比记录里偏高。' }],
      },
    },
  ],
}
