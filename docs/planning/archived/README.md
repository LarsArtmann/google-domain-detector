# Archived plans

Closed plan history — every task row carries an inline verdict (done at
hash / verified evidence / superseded / won't-implement / routed pointer).
Do not reopen archived plans; the living docs (TODO_LIST.md, ROADMAP.md,
docs/census.md) own open work. check-rows --strict + bare-rows were clean
BEFORE each move (archive gate).

## 2026-11-14 bulk archive manifest (docs-health pass)

Three fully-executed plans from 2026-11-10, classified ARCHIVE: v0.1.0
shipped 2026-11-10 (T6/E2), the scale-readiness spine landed (streaming,
resume, hardening, doctor, nom), the census chain moved to the living
TODO_LIST rows (waves B/C, T14, T15 remain deliberately open there), and
every superseded row (FormatsForShape, NXDOMAIN pre-sweep, `-o`) carries
its successor pointer.

| File (in archived/)                                         | Classification | Deciding reason                                                                                |
| ----------------------------------------------------------- | -------------- | ---------------------------------------------------------------------------------------------- |
| 2026-11-10_00-50_pareto-plan-ship-v0.1.0-scale-readiness.md | ARCHIVE        | All 27 L1 tasks landed/decided (v0.1.0 shipped same day; verdicts cite commits)                |
| 2026-11-10_01-18_pareto-plan-v2-journal-scale-deghost.md    | ARCHIVE        | All 27 L1 tasks landed/decided (scanstore/read-model/replay work shipped; T9 resume = 3674285) |
| 2026-11-10_09-21_unified-execution-v3-ship-then-census.md   | ARCHIVE        | E1-E13/E18-E33 landed; E14/E15/E16/E17/E32/E34/E35 routed to the living TODO_LIST/ROADMAP rows |

Still open in docs/planning/: the census master plan
(2026-11-10_06-18, waves B/C + poisonfix + T15 final) and the census
closeout execution canon (2026-11-14_00-42 SUPERB plan).
