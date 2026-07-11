# AP-105 90-day content calendar

Source of truth for templates: `frontend/src/liveops/calendar/templates.ts`.

## Templates

| Id | Kind | Miss policy |
|---|---|---|
| tpl.observation_week | observation_week | rerun_window |
| tpl.city_research | city_research | catalog_unlock |
| tpl.knowledge | knowledge_challenge | compensate_soft |
| tpl.photo | photo_theme | rerun_window |
| tpl.welfare | welfare_day | compensate_soft |

## Safety

- No night outdoor requirement, extreme-weather gate, cross-city chase, or rare-animal aggregation.
- Missing an event never locks core progression (`missDoesNotLockCore`).

## Ops

- Instances still rely on AP-081 LiveOps state machine.
- Publish via content/LiveOps definitions; rollback notes on each template.
