import { useI18n } from '../../../i18n'
/** 动物福利提示（#205）—— 不鼓励现实投喂/追逐 */
export default function WelfareNotice() {
  const { t } = useI18n()
  return (
    <p
      role="note"
      style={{
        fontSize: 11,
        lineHeight: 1.4,
        color: 'var(--ink-3, #8B6B55)',
        margin: '8px 16px 0',
      }}
    >
      {t('welfare.notice')}
    </p>
  )
}
