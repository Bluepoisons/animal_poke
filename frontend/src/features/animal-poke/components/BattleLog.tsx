interface BattleLogProps {
  lines: string[]
}

export default function BattleLog({ lines }: BattleLogProps) {
  return (
    <div className="ap-battle-log" aria-live="polite">
      <div className="ap-battle-log__title">手账战斗日志</div>
      {lines.map((line, index) => (
        <div key={`${index}-${line}`}>{line}</div>
      ))}
    </div>
  )
}
