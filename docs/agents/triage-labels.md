# Triage Labels

The skills speak in terms of five canonical triage roles. This file maps those roles to the actual label strings used in this repo's issue tracker (GitHub Issues — see `issue-tracker.md`).

| Label in mattpocock/skills | Label in our tracker | Meaning                                  |
| -------------------------- | -------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer needs to evaluate this issue  |
| `needs-info`               | `needs-info`         | Waiting on reporter for more information |
| `ready-for-agent`          | `ready-for-agent`    | Fully specified, ready for an AFK agent  |
| `ready-for-human`          | `ready-for-human`    | Requires human implementation            |
| `wontfix`                  | `wontfix`            | Will not be actioned                     |

When a skill mentions a role (e.g. "apply the AFK-ready triage label"), use the corresponding label string from this table.

Edit the right-hand column to match whatever vocabulary you actually use.

## 이 레포의 현재 상태

**`wontfix` 만 GitHub 에 실제로 존재한다.** 나머지 네 개는 아직 만들지 않았다 — 일부러다.
쓸지 모르는 라벨을 미리 만들면 라벨 목록만 늘고 아무 이슈도 달리지 않는다.

`/triage` 를 처음 돌릴 때 그 자리에서 만들면 된다:

```bash
gh label create needs-triage    --description "Maintainer needs to evaluate this issue"
gh label create needs-info      --description "Waiting on reporter for more information"
gh label create ready-for-agent --description "Fully specified, ready for an AFK agent"
gh label create ready-for-human --description "Requires human implementation"
```

## 트리아지 대상이 아닌 라벨

- **`trigger-waiting`** — 조건이 충족될 때까지 **의도적으로** 보류된 이슈. 방치가 아니고,
  정보가 부족한 것도 아니다. 판단은 이미 끝났고 실행 시점만 남았다.
  `needs-triage`(평가 필요)나 `needs-info`(정보 부족)와 헷갈리면 안 된다 —
  **`trigger-waiting` 이 붙은 이슈는 트리아지 큐에 넣지 않는다.**
  트리거 조건은 각 이슈 본문에 체크박스로 적혀 있다.
