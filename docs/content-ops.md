# Content Operations Quick Reference (AP-110)

## Authoring workflow

1. Define content in the appropriate catalog (species pack, quest catalog, etc.).
2. Add i18n keys for all locales (`frontend/src/i18n/locales/`).
3. Run the manifest gate: `node scripts/content-manifest-gate.mjs`
4. The gate verifies:
   - Species packs are mirrored frontend ↔ backend
   - All i18n keys exist in every locale
   - Cross-references (species in quests, battles, shop) are valid
   - Reward items exist in the shop inventory
   - Effect key namespaces are approved

## Publishing a new species

1. Add to `backend/internal/speciespack/builtin.go` with `ContentID`, `Status`, `Certification`
2. Mirror in `frontend/src/species/packs.ts` with identical `contentId`, `status`
3. Set `status: StatusCatalogOnly` if not yet capturable
4. Add i18n keys for names, habitat, observation tips, welfare notes
5. Run `node scripts/content-manifest-gate.mjs`

## Publishing a new quest

1. Add to `backend/internal/questcatalog/catalog.go` as a `Def` struct
2. Ensure `Species` references valid species IDs
3. Ensure `Reward` items exist in `frontend/src/shop/constants.ts`
4. Use approved effect key namespaces: `flag:`, `rel:`, `clue:`, `knowledge:`, `reward:`, `item:`, `gold:`, `exp:`, `stamina:`

## Incomplete locales

The manifest gate fails CI when any locale is missing keys present in the reference locale (zh). Before publishing, either:
- Complete the missing translations, or
- Mark the locale as hidden in the feature flags until translations are ready
