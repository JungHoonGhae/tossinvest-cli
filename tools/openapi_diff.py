#!/usr/bin/env python3
"""공식 Open API spec 두 버전을 대조해 **실제** 변경만 뽑는다.

왜 필요한가
-----------
매번 즉석 스크립트로 diff 를 보다가 같은 함정을 두 번 밟았다:

- 2026-08-03: `paths` 만 보고 `components` 변경 6건을 통째로 놓쳤다. 거기에 "US 지정가도
  호가 단위 검증 대상" 이 들어 있었다.
- 2026-08-07: 스키마 덤프에서 `allOf` 를 안 펼쳐 필드 타입이 비어 보였고, 객체를
  문자열로 착각해 구현이 런타임에 깨졌다.

둘 다 도구가 없어서 난 일이다. 한 곳에 박아두면 다음 사람이 다시 밟지 않는다.

`git show` 로 텍스트 diff 를 보면 스키마가 알파벳 순으로 재배치될 때 추가·삭제가
수백 줄 뜬다. 여기서는 **집합 차분**만 내므로 그 노이즈가 없다.

사용
----
    python3 tools/openapi_diff.py                    # HEAD 의 스냅샷 vs 그 직전 커밋
    python3 tools/openapi_diff.py --rev 7fd9486      # 그 커밋 vs 그 부모
    python3 tools/openapi_diff.py --schema ShortSellingResponse   # 스키마 한 그루 펼치기
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys

SPEC = "docs/migration/openapi.latest.json"


def load(rev: str | None) -> dict:
    if rev is None:
        with open(SPEC, encoding="utf-8") as fh:
            return json.load(fh)
    return json.loads(subprocess.check_output(["git", "show", f"{rev}:{SPEC}"]))


def methods(entry: dict) -> list[str]:
    return [m for m in entry if m in ("get", "post", "put", "patch", "delete")]


def resolve(schema: dict) -> tuple[dict, bool]:
    """allOf·oneOf 를 펼쳐 (스키마, nullable) 로 만든다.

    이 spec 은 $ref 를 두 가지로 감싼다:

      allOf: [{$ref}]              — $ref 에 description 을 덧붙일 때
      oneOf: [{$ref}, {type:null}] — nullable 한 $ref

    둘 다 `$ref` 키가 한 겹 아래에 있어서, properties 를 얕게 훑으면 **타입 없는 필드**로
    보인다. 2026-08-07 에 기관 세부 분류를 그렇게 놓쳐 객체를 문자열로 구현했다.
    """
    nullable = False
    for key in ("allOf", "oneOf", "anyOf"):
        if key not in schema:
            continue
        merged = {k: v for k, v in schema.items() if k not in ("allOf", "oneOf", "anyOf")}
        for part in schema[key]:
            if part.get("type") == "null":
                nullable = True
                continue
            merged.update({k: v for k, v in part.items() if k not in merged})
        inner, inner_null = resolve(merged)
        return inner, nullable or inner_null
    return schema, nullable


def type_of(schema: dict) -> tuple[str, bool]:
    s, nullable = resolve(schema)
    if "$ref" in s:
        return s["$ref"].split("/")[-1], nullable
    if s.get("type") == "array":
        inner, _ = type_of(s.get("items", {}))
        return "[]" + inner, nullable
    t = s.get("type")
    if isinstance(t, list):  # ["string", "null"]
        return "|".join(x for x in t if x != "null"), nullable or "null" in t
    return t or "?", nullable


def dump_schema(schemas: dict, name: str, depth: int = 1, seen: set[str] | None = None) -> None:
    seen = seen if seen is not None else set()
    if name in seen or depth > 5 or name not in schemas:
        return
    seen.add(name)
    for key, raw in schemas[name].get("properties", {}).items():
        t, nullable = type_of(raw)
        print("  " * depth + f"{key}: {t}" + ("   (nullable)" if nullable else ""))
        child = t.lstrip("[]")
        if child in schemas:
            dump_schema(schemas, child, depth + 1, seen)


def fmt_param(spec: dict, param: dict) -> str:
    if "$ref" in param:
        key = param["$ref"].split("/")[-1]
        param = spec.get("components", {}).get("parameters", {}).get(key, param)
    t, _ = type_of(param.get("schema", {}))
    default = param.get("schema", {}).get("default")
    bits = [param.get("in", "?")]
    if param.get("required"):
        bits.append("req")
    if default is not None:
        bits.append(f"default={default}")
    return f"{param.get('name','?')}:{t}({','.join(bits)})"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--rev", help="이 커밋과 그 부모를 대조 (기본: 워킹트리 vs HEAD)")
    ap.add_argument("--schema", help="스키마 한 그루를 allOf 까지 펼쳐 출력")
    args = ap.parse_args()

    new = load(None if args.rev is None else args.rev)
    if args.schema:
        dump_schema(new.get("components", {}).get("schemas", {}), args.schema)
        return 0

    old = load(f"{args.rev}^" if args.rev else "HEAD")

    ov, nv = old.get("info", {}).get("version"), new.get("info", {}).get("version")
    op, np = set(old.get("paths", {})), set(new.get("paths", {}))
    os_, ns = (
        set(old.get("components", {}).get("schemas", {})),
        set(new.get("components", {}).get("schemas", {})),
    )

    print(f"version  {ov} → {nv}")
    print(f"paths    {len(op)} → {len(np)}   (+{len(np - op)} / -{len(op - np)})")
    print(f"schemas  {len(os_)} → {len(ns)}   (+{len(ns - os_)} / -{len(os_ - ns)})")

    if np - op:
        print("\n[신규 경로]")
        for p in sorted(np - op):
            for m in methods(new["paths"][p]):
                op_def = new["paths"][p][m]
                # 파라미터도 $ref 로 온다(#/components/parameters/KrSymbol). 풀지 않으면
                # 이름이 "?" 로 찍혀 호출법을 알 수 없다.
                params = ", ".join(
                    fmt_param(new, q) for q in op_def.get("parameters", [])
                )
                print(f"  {m.upper():6} {p}")
                print(f"         {op_def.get('summary','')}")
                if params:
                    print(f"         params: {params}")

    for label, gone in (("[삭제 경로]", op - np), ("[삭제 스키마]", os_ - ns)):
        # 삭제는 breaking change 다. 없으면 없다고 찍어서, 안 본 것과 구분한다.
        print(f"\n{label} {sorted(gone) if gone else '없음'}")

    # 경로·스키마 개수가 그대로여도 **필드의 의미**가 바뀔 수 있다. 2026-08-12 에
    # Commission.commissionRate 의 단위가 "퍼센트" → "소수 비율" 로 뒤집혔는데
    # (같은 요율을 100배 다르게 쓴다) 집합 차분은 +0/-0 이라 no-op 으로 보였다.
    # 타입이 안 변하는 의미 변경은 여기서만 잡힌다.
    drift = []
    for name in sorted(os_ & ns):
        oldp = old["components"]["schemas"][name].get("properties", {})
        newp = new["components"]["schemas"][name].get("properties", {})
        for field in sorted(set(oldp) & set(newp)):
            for attr in ("description", "example", "type", "format"):
                a, b = oldp[field].get(attr), newp[field].get(attr)
                if a != b:
                    drift.append((name, field, attr, a, b))
    if drift:
        print(f"\n[필드 의미 변경 {len(drift)}건] — 개수는 그대로지만 뜻이 바뀐 자리다")
        for name, field, attr, a, b in drift:
            print(f"  {name}.{field}  ({attr})")
            print(f"    - {a}")
            print(f"    + {b}")

    # 스키마 필드뿐 아니라 **오퍼레이션 설명**도 동작 규칙을 담는다. 2026-08-12 에
    # createConditionalOrder 설명에 "국내는 KRX 정규장에서만 발동" 이 추가됐는데,
    # 위의 필드 순회로는 안 잡힌다(오퍼레이션 레벨이라서). 사용자가 조건주문을
    # 걸어두고 왜 발동 안 하는지 묻게 되는 종류의 규칙이다.
    op_drift = []
    for path in sorted(op & np):
        for m in methods(new["paths"][path]):
            if m not in old["paths"][path]:
                continue
            for attr in ("summary", "description"):
                a = old["paths"][path][m].get(attr)
                b = new["paths"][path][m].get(attr)
                if a != b:
                    op_drift.append((m.upper(), path, attr, a or "", b or ""))
    if op_drift:
        print(f"\n[오퍼레이션 설명 변경 {len(op_drift)}건] — 동작 규칙이 여기 적힌다")
        for m, path, attr, a, b in op_drift:
            print(f"  {m} {path}  ({attr})")
            # 통짜 비교는 읽을 수 없다. 줄 단위로 갈라 새로 생긴 줄만 보여준다.
            added = [ln for ln in b.split("\n") if ln.strip() and ln not in a.split("\n")]
            removed = [ln for ln in a.split("\n") if ln.strip() and ln not in b.split("\n")]
            for ln in removed:
                print(f"    - {ln.strip()[:110]}")
            for ln in added:
                print(f"    + {ln.strip()[:110]}")

    if ns - os_:
        print(f"\n[신규 스키마 {len(ns - os_)}개]")
        print("  " + ", ".join(sorted(ns - os_)))
        print("\n  → 필드까지 보려면: python3 tools/openapi_diff.py --schema <이름>")
    return 0


if __name__ == "__main__":
    sys.exit(main())
