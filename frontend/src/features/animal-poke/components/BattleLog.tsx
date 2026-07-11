import { useI18n } from '../../../i18n'
interface BattleLogProps {
  lines: string[]
}

export default function BattleLog({ lines }: BattleLogProps) {
  const { t } = useI18n()
  return (
    <div className="ap-battle-log" aria-live="polite">
      <div className="ap-battle-log__title">{t('battle.log')}</div>
      {lines.map((line, index) => (
        <div key={`${index}-${line}`}>{line}</div>
      ))}
    </div>
  )
}
