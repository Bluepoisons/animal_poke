import type { ScreenId } from '../data/types'
import { useI18n } from '../../../i18n'

interface PhoneFrameProps {
  variant: ScreenId
  children: React.ReactNode
}

export default function PhoneFrame({ variant, children }: PhoneFrameProps) {
  const { t } = useI18n()
  return (
    <div className={`ap-phone ap-phone--${variant}`} data-phone-frame={variant} aria-label={t('phone.label')}>
      {children}
    </div>
  )
}
