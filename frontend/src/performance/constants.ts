/** Performance / battery budgets (AP-054) */

/** Battery level (0–1) below which we force low-power mode */
export const LOW_BATTERY_LEVEL = 0.20

/** Long-task / frame budget (ms) — jank signal if exceeded */
export const FRAME_BUDGET_MS = 32

/** Rolling jank sample window */
export const JANK_SAMPLE_SIZE = 30

/** Fraction of frames over budget that triggers low-end mode */
export const JANK_RATIO_THRESHOLD = 0.35

/** Default upload max edge (px) */
export const UPLOAD_MAX_EDGE_DEFAULT = 1280
export const UPLOAD_MAX_EDGE_SAVER = 720
export const UPLOAD_MAX_EDGE_LOW = 640

/** JPEG quality */
export const UPLOAD_QUALITY_DEFAULT = 0.85
export const UPLOAD_QUALITY_SAVER = 0.7
export const UPLOAD_QUALITY_LOW = 0.55

/** Continuous scan interval (ms) */
export const CONTINUOUS_SCAN_MS = 2500
export const CONTINUOUS_SCAN_MS_SAVER = 5000

export const PERF_STORAGE_KEY = 'animal_poke_perf_mode'
