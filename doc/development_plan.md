# Open Drug Bridge 개발 계획

## 1. 프로젝트 목표

`Open Drug Bridge`는 EMR 데이터베이스에서 약품 관련 정보만 읽어 로컬 API와 Web UI로 제공하는 Windows용 오픈소스 도구다.

v0.1은 Eghis EMR PostgreSQL 스키마만 지원한다. 이후 다른 EMR을 adapter 방식으로 추가할 수 있게 설계한다.

주요 원칙:

- 약품 관련 데이터만 제공한다.
- PostgreSQL 읽기 전용 계정만 사용한다.
- 환자 개인정보 API를 만들지 않는다.
- Windows 사용자가 단일 실행 파일로 쉽게 실행할 수 있게 한다.
- 서버 설정 스크립트는 EMR별 폴더로 격리한다.

## 2. 이름과 배포 형태

권장 이름:

- 오픈소스 repository: `open-drug-bridge`
- v0.1 앱 이름: `Drug Storage Bridge`
- Windows 실행 파일: `drug-storage-bridge.exe`
- 서버 설정 스크립트 prefix: `drug-storage-bridge_eghis`

v0.1 배포물:

```text
open-drug-bridge/
  drug-storage-bridge.exe
  config.example.yaml
  README.md
  doc/
  scripts/
    eghis/
```

## 3. 기술 스택

언어는 Go만 사용한다.

선택 이유:

- Windows 단일 실행 파일 배포가 쉽다.
- 별도 Python runtime, Node runtime 없이 실행 가능하다.
- 서버 API, 정적 Web UI embed, PostgreSQL 접속을 한 바이너리에서 처리할 수 있다.
- 병의원 Windows PC 환경에 배포하기 적합하다.

권장 라이브러리:

```text
HTTP router: github.com/go-chi/chi/v5
PostgreSQL: github.com/jackc/pgx/v5
Config: gopkg.in/yaml.v3
Windows credential: DPAPI wrapper 또는 Windows Credential Manager wrapper
XLSX export: github.com/xuri/excelize/v2
Logging: log/slog
```

Frontend는 v0.1에서 단순 정적 Web UI로 시작한다.

권장:

```text
HTML/CSS/vanilla JS
또는 HTMX + Alpine.js
```

빌드 산출물은 Go `embed`를 사용해 Web UI 파일을 실행 파일에 포함한다.

## 4. 범위

### v0.1 포함

- Eghis PostgreSQL adapter
- Web UI 최초 설정 화면
- PostgreSQL 연결 테스트
- DB 접속 설정 저장
- 비밀번호 암호화 저장
- 약품 검색
- 약품 상세
- 일반약 재고 계산
- 향정/마약류 NIMS 재고 계산
- 최근 처방량 집계
- 주문 필요량 계산
- XLSX export
- Windows 서버용 읽기 전용 계정 생성 스크립트

### v0.1 제외

- 환자별 처방 내역 조회 API
- 환자 개인정보 제공
- EMR 데이터 수정
- 다중 병원 중앙 서버
- 클라우드 동기화
- 인증/권한 관리 서버
- 전체 PostgreSQL 물리 백업/복구
- Linux/macOS 공식 배포

## 5. 보안 원칙

### DB 계정

앱은 읽기 전용 PostgreSQL 계정만 사용한다.

권한은 최소화한다.

- `CONNECT` on database
- `USAGE` on schema
- 약품 계산에 필요한 테이블에만 `SELECT`

### 비밀번호 저장

Web UI에서 입력받은 DB 비밀번호는 평문 저장하지 않는다.

Windows에서는 다음 중 하나를 사용한다.

- Windows DPAPI
- Windows Credential Manager

설정 파일에는 비밀번호 본문 대신 암호화된 blob 또는 credential key만 저장한다.

예시:

```yaml
server:
  host: 127.0.0.1
  port: 3987

emr:
  adapter: eghis

database:
  host: 192.168.0.10
  port: 5432
  name: postgres
  user: eghis_drug_ro
  password_ref: windows-credential:drug-storage-bridge/eghis_drug_ro
  sslmode: disable
```

### 개인정보

API 응답에는 기본적으로 다음 값을 포함하지 않는다.

- 환자명
- 주민등록번호
- 전화번호
- 주소
- 수진번호
- 상세 환자별 처방 목록

처방량 집계는 약품코드, 약품명, 기간, 수량 단위로만 반환한다.

### 보안 책임 고지

공개 배포 전 README에는 보안 관리 책임 범위를 명확히 적는다.

초안:

```text
Open Drug Bridge는 EMR PostgreSQL 데이터베이스에 대한 읽기 전용 약품 조회 도구입니다.
이 도구는 서버의 보안 정책, PostgreSQL 계정 관리, 네트워크 접근 제어, 백업 파일 보관 정책을 대신 책임지지 않습니다.
DB 계정 생성, pg_hba.conf 변경, 방화벽 설정, 백업/복구 정책 수립과 운영 책임은 해당 서버의 관리자 및 소유자에게 있습니다.
운영자는 배포 전 각 기관의 보안 정책과 개인정보보호 규정을 검토해야 합니다.
```

README에는 특히 다음을 강조한다.

- 앱 런타임에는 읽기 전용 DB 계정 사용을 권장한다.
- 서버 설정/부트스트랩 스크립트는 서버 관리자 권한으로 실행되며, 실행 책임은 운영자에게 있다.
- `pg_hba.conf` 임시 trust 부트스트랩은 로컬 서버에서만 제한적으로 사용해야 한다.
- 백업 파일에는 민감 데이터가 포함될 수 있으므로 저장 위치, 접근 권한, 보관 기간은 운영자가 관리해야 한다.

### 비개발자 사용자를 전제로 한 보안 설계

이 프로젝트는 개발자나 DB 관리자가 아닌 병의원 실무자가 사용할 수 있다. 따라서 보안은 "문서로 경고"하는 수준이 아니라, 기본 동작과 UI 자체가 안전한 선택을 하도록 설계한다.

#### 기본값

| 항목 | 기본값 | 이유 |
| --- | --- | --- |
| 서버 listen host | `127.0.0.1` | 같은 PC에서만 접근 가능하게 함 |
| LAN 공개 | 꺼짐 | 실수로 병원망 전체 또는 인터넷에 노출되는 것을 방지 |
| DB 권한 | 읽기 전용 | 데이터 수정/삭제 방지 |
| PostgreSQL superuser 저장 | 금지 | 앱 탈취 시 DB 전체 장악 방지 |
| 환자정보 API | 미제공 | 개인정보 노출면 축소 |
| 로그 민감정보 | 마스킹 | DSN, 비밀번호, 환자 식별자 유출 방지 |
| 백업 기능 | 약품 데이터 export만 기본 | 전체 DB 백업 오남용 방지 |

LAN 공개 기능은 `host: 0.0.0.0` 설정으로 허용한다. 다만 기본값은 반드시 `127.0.0.1`이고, Web UI에서는 고급 설정으로 분리한다.

`0.0.0.0` 사용을 허용하는 경우:

- 같은 병원 내부망에서만 접근해야 하는 경우
- Tailscale, WireGuard 같은 VPN으로 인증된 장비끼리만 접근해야 하는 경우
- 서버 PC가 별도 방화벽으로 보호되고, 허용된 원내 IP 대역에서만 접근 가능한 경우

`0.0.0.0` 사용을 금지하거나 강하게 경고해야 하는 경우:

- 공유기/방화벽에서 인터넷 포트포워딩을 설정하려는 경우
- 공인 IP에서 직접 접근 가능하게 만들려는 경우
- 원격 접속을 위해 임의 포트를 외부에 개방하려는 경우
- 접근 제어 없이 병원 Wi-Fi 전체에 노출하려는 경우

UI에서 `0.0.0.0`을 켤 경우 다음 조건을 모두 만족해야 한다.

- 명시적 확인 checkbox
- 방화벽/네트워크 노출 경고 표시
- Tailscale/VPN 또는 원내망 한정 사용 권고 표시
- 인터넷 포트포워딩 금지 문구 표시
- "인터넷 포트포워딩을 설정하지 않았습니다" 확인 checkbox
- "VPN 또는 원내망에서만 사용할 것입니다" 확인 checkbox
- 관리자 비밀번호 재입력 또는 로컬 confirmation phrase 입력
- 설정 화면에 현재 접근 URL과 노출 상태 표시

confirmation phrase 예:

```text
원내망 또는 VPN에서만 사용
```

UI 경고 문구 초안:

```text
주의: 0.0.0.0으로 실행하면 다른 PC에서도 이 Web UI에 접근할 수 있습니다.
인터넷 공유기/방화벽에서 포트포워딩을 설정하지 마세요.
외부 접속이 필요하면 Tailscale/WireGuard 같은 VPN을 사용하거나 원내망 내부에서만 접근하도록 제한하세요.
```

#### 연결 설정 UI 안전장치

Setup 화면은 사용자가 무심코 강한 권한 계정을 입력하지 않도록 검사한다.

검사 항목:

- 입력 계정이 `postgres`, `admin`, `root`, `superuser`처럼 보이면 경고
- 연결 후 `SELECT rolsuper FROM pg_roles`로 superuser 여부 확인
- `INSERT/UPDATE/DELETE` 권한 보유 여부를 가능한 범위에서 점검
- 읽기 전용 권한이 아니면 저장 전 경고
- 약품 조회에 필요한 최소 테이블 권한 누락 시 어떤 권한이 부족한지 표시

저장 허용 정책:

```text
정상: 읽기 전용 계정 -> 저장 허용
주의: 과도한 SELECT 권한 -> 경고 후 저장 허용 가능
차단: superuser 계정 -> 기본 저장 차단
차단 예외: 개발자 모드 설정 파일에서만 허용, UI에서는 허용하지 않음
```

#### 비밀번호와 설정 파일

비밀번호는 평문 저장하지 않는다.

Windows 구현 원칙:

- Windows DPAPI 또는 Credential Manager 사용
- 설정 파일에는 credential key만 저장
- 로그와 오류 메시지에 password, DSN full string 출력 금지
- export/import 설정 기능을 만들 경우 비밀번호는 내보내지 않음

설정 파일 권한:

- `%APPDATA%\OpenDrugBridge\config.yaml`
- 생성 시 현재 Windows 사용자만 읽기/쓰기 가능하도록 ACL 제한을 시도
- ACL 설정 실패 시 UI에 경고 표시

#### 로컬 Web UI 보호

v0.1은 기본적으로 로컬 앱이므로 별도 로그인은 두지 않는다. 대신 네트워크 노출을 막는 데 집중한다.

요구사항:

- 기본 bind는 `127.0.0.1`
- `Origin`/`Host` header 검사
- localhost가 아닌 Host 접근은 기본 차단
- CSRF 위험을 줄이기 위해 설정 변경 API는 POST + one-time setup token 사용
- setup token은 앱 시작 시 메모리에 생성하고 Web UI에만 주입
- 설정 저장, reset, backup export 같은 민감 action은 token 필수

LAN 공개 모드가 추가될 경우:

- 별도 local admin password 또는 random access token 필요
- 최초 공개 시 token을 화면에만 표시
- README에는 reverse proxy 없이 인터넷 공개 금지 명시
- README에는 공유기 포트포워딩 금지 명시
- 권장 원격 접근 방식은 Tailscale/WireGuard 같은 VPN으로 제한
- 원내망 사용 시 Windows 방화벽에서 허용 IP 대역을 제한하도록 안내

LAN 공개 모드의 설정 예:

```yaml
server:
  host: 0.0.0.0
  port: 3987
  access_token_required: true
  allowed_network_hint: "clinic-lan-or-vpn-only"
```

LAN 공개 모드는 최소한 다음 보호장치를 요구한다.

- Web UI 접근 token
- 설정 변경 API token
- Host header 검사
- 민감 action 재확인
- 시작 로그와 UI header에 "LAN 공개 모드" 표시
- `/health`를 제외한 API는 token 없이는 접근 불가

향후 더 안전한 구현을 위해 `allowed_cidrs` 설정을 제공한다.

```yaml
server:
  host: 0.0.0.0
  port: 3987
  allowed_cidrs:
    - 192.168.0.0/24
    - 100.64.0.0/10   # Tailscale CGNAT range
```

`allowed_cidrs`가 설정되면 해당 대역 외 요청은 앱 레벨에서 차단한다.

#### 원격 접속 권장 방식

원격 접속은 앱 자체가 인터넷에 직접 노출되는 방식이 아니라, 인증된 사설 네트워크 위에서만 허용하는 방식을 권장한다.

권장 순위:

| 순위 | 방식 | 설명 | 문서화 수준 |
| --- | --- | --- | --- |
| 1 | Tailscale | WireGuard 기반 mesh VPN. 포트포워딩 없이 인증된 장비끼리 접근 | README에서 1순위 권장 |
| 2 | ZeroTier | 가상 사설망 방식. 네트워크 ID에 가입된 장비끼리 접근 | 대안으로 안내 |
| 3 | NetBird | WireGuard 기반 mesh VPN. 셀프호스트 가능 | 고급 대안으로 안내 |
| 4 | 병원 기존 VPN 장비 | Fortinet, Sophos, OpenVPN, Windows VPN 등 | 병원 전산 담당자가 있을 때 안내 |
| 5 | Cloudflare Tunnel + Access | 브라우저 기반 Zero Trust 접근 | 고급 사용자용, 기본 권장 아님 |

기본 권장 문구:

```text
원격 접속이 필요하면 공유기 포트포워딩을 사용하지 마세요.
Tailscale, ZeroTier, NetBird, 병원 기존 VPN처럼 인증된 장비만 접근 가능한 사설 네트워크를 사용하세요.
```

앱은 특정 VPN 제품에 종속되지 않는다. 대신 다음 보호장치를 제공한다.

- `host: 0.0.0.0` LAN 공개 모드
- access token
- Host/Origin 검사
- `allowed_cidrs`
- LAN 공개 경고 UI
- 설정 변경 API token
- 보안 상태 표시

Tailscale을 사용할 때의 예:

```yaml
server:
  host: 0.0.0.0
  port: 3987
  access_token_required: true
  allowed_cidrs:
    - 100.64.0.0/10
```

원내망에서만 사용할 때의 예:

```yaml
server:
  host: 0.0.0.0
  port: 3987
  access_token_required: true
  allowed_cidrs:
    - 192.168.0.0/24
```

금지 문구:

```text
공유기 포트포워딩, 공인 IP 직접 공개, 방화벽 전체 허용으로 이 앱을 인터넷에 노출하지 마세요.
이 앱은 병원 내부망 또는 VPN 내부에서 사용하도록 설계되었습니다.
```

Web UI 보안 상태 배너:

| 상태 | 표시 |
| --- | --- |
| `127.0.0.1` | 안전: 이 PC에서만 접근 가능합니다. |
| `0.0.0.0` + token + CIDR | 주의: 원내망/VPN에서 접근 가능합니다. 허용 대역을 확인하세요. |
| `0.0.0.0` + token only | 경고: 다른 PC에서 접근 가능합니다. VPN/방화벽 제한을 확인하세요. |
| `0.0.0.0` + token 없음 | 위험: 네트워크에 노출되어 있습니다. 이 설정은 허용하지 않습니다. |

v0.1 저장 정책:

```text
127.0.0.1:
  저장 허용

0.0.0.0 + access token:
  경고 후 저장 허용

0.0.0.0 + access token + allowed_cidrs:
  권장 LAN/VPN 공개 설정으로 저장 허용

0.0.0.0 + access token 없음:
  저장 차단
```

README의 원격 접속 섹션은 다음 순서로 작성한다.

1. 기본은 같은 PC에서만 사용
2. 여러 PC에서 쓰려면 원내망 또는 VPN 사용
3. Tailscale 설정 예시
4. ZeroTier/NetBird 대안
5. Windows 방화벽에서 허용 IP 제한
6. 포트포워딩 금지
7. 문제가 생기면 다시 `127.0.0.1`로 되돌리는 방법

#### API 데이터 최소화

API 응답은 약품 중심으로 제한한다.

허용:

- 약품코드
- 약품명
- 성분명
- 용량
- 처방량 집계
- 재고량
- 입고/사용/폐기/양도 집계
- 기간별 사용량

금지:

- 환자명
- 주민등록번호
- 전화번호
- 주소
- 수진번호
- 진료기록 원문
- 환자별 처방 상세 목록

불가피하게 내부 계산에 환자/수진 테이블을 join해야 하는 경우에도 응답 DTO에서 제거한다.

#### 로그 정책

로그는 문제 해결에 필요한 최소 정보만 남긴다.

마스킹 대상:

- DB password
- full DSN
- host가 외부망인 경우 일부 마스킹
- 환자 식별자
- 수진번호
- SQL parameter 중 개인정보 가능성이 있는 값

로그 수준:

```text
INFO: 서버 시작, 포트, adapter, 성공/실패 요약
WARN: 권한 과다, 권한 부족, LAN 공개 경고
ERROR: 오류 코드와 요약. 민감 parameter 제외
DEBUG: 기본 비활성화. 개발자 모드에서만 사용
```

로그 보관:

- 기본 14일 또는 10MB rolling
- UI에서 로그 다운로드 기능은 기본 제공하지 않음
- 제공하더라도 민감정보 마스킹 후 export

#### 서버 스크립트 안전장치

`scripts/eghis/bootstrap_eghis_drug_readonly.ps1`는 실수로 인증을 넓게 여는 것을 방지해야 한다.

필수 안전장치:

- 관리자 권한 PowerShell인지 확인
- `pg_hba.conf` 백업 없이는 진행 금지
- 임시 trust는 `127.0.0.1/32`, `::1/128`, `postgres` 사용자만 허용
- `0.0.0.0/0 trust`, `all all trust` 라인 생성 금지
- 작업 완료 후 임시 trust 라인이 남아 있으면 실패 처리
- 실패 시 백업 복구 시도
- 복구 실패 시 화면에 수동 복구 경로와 백업 파일명 표시
- 최종 read-only 계정으로 접속 테스트
- 최종 read-only 계정의 쓰기 권한 테스트가 실패해야 성공으로 간주

스크립트는 interactive 확인을 요구한다.

예:

```text
이 스크립트는 PostgreSQL 인증 설정(pg_hba.conf)을 임시 변경합니다.
임시 trust는 로컬 postgres 접속에만 적용되며 작업 후 제거됩니다.
계속하려면 서버 이름을 입력하세요: ______
```

자동화용 `-Force` 옵션은 v0.1에서 만들지 않는다.

#### PostgreSQL 부트스트랩 고위험 구간 계획

서버 스크립트 중 가장 위험한 단계는 PostgreSQL 인증을 임시로 완화해 읽기 전용 계정을 생성하는 과정이다. 이 단계는 관리자 권한이 필요하고, `pg_hba.conf` 수정, 서비스 reload/restart, PostgreSQL 버전 차이, 설치 경로 차이를 모두 다뤄야 한다.

따라서 bootstrap 스크립트는 일반 앱 기능이 아니라 "서버 유지보수 도구"로 분리하고, 다음 원칙을 따른다.

### 위험 요소

| 위험 | 설명 | 대응 |
| --- | --- | --- |
| 인증 설정 훼손 | `pg_hba.conf`를 잘못 수정하면 EMR 접속 장애 가능 | 원본 백업, diff 표시, 복구 자동화 |
| 임시 trust 잔존 | 작업 후 local trust가 남으면 보안 위험 | 종료 전 trust 제거 검증, 남아 있으면 실패 처리 |
| 넓은 trust 생성 | `0.0.0.0/0 trust` 같은 치명적 설정 위험 | 스크립트에서 생성 금지, 패턴 검사 |
| 서비스 restart 장애 | PostgreSQL 재시작 실패 시 EMR 장애 가능 | reload 우선, restart 전 경고, maintenance window 안내 |
| 비표준 설치 경로 | PostgreSQL이 기본 경로에 없을 수 있음 | 자동 탐색 + 수동 지정 지원 |
| 구버전 호환성 | PostgreSQL 8~9대 병의원 존재 가능 | 버전 감지 후 auth method/SQL 분기 |
| 32-bit 설치 | 오래된 Windows 서버에서 가능 | Program Files (x86) 탐색 |
| 다중 인스턴스 | 여러 PostgreSQL 서비스가 존재할 수 있음 | 서비스 선택 UI/프롬프트 |
| encoding/locale | 구버전 Windows/DB에서 한글 출력 문제 | 스크립트 내부 식별자는 ASCII 사용 |

### PostgreSQL 버전 호환성과 md5 인증

PostgreSQL 8~9대까지 고려한다.

버전 감지:

```sql
SHOW server_version;
SHOW server_version_num;
```

호환성 분기:

| PostgreSQL 버전 | 인증 방식 기본값 | 비고 |
| --- | --- | --- |
| 8.x | `md5` 또는 `password` | 오래된 병의원 서버에서 가능성 있음. `CREATE USER`/`ALTER USER` 문법 제한 확인 필요 |
| 9.x | `md5` | 구버전 Eghis 환경의 기본 후보. `scram-sha-256` 미지원 |
| 10~13 | `md5`, 일부 환경 `scram-sha-256` 가능 | 서버 설정 확인 필요 |
| 14+ | `scram-sha-256` 권장 | 가능하면 `scram-sha-256` 사용 |

구버전 Eghis 서버는 `pg_hba.conf`에 `md5`를 쓰고 있을 가능성이 높다. 사용자가 기억하는 "mdf5" 형태는 PostgreSQL의 `md5` 인증일 가능성이 크다.

`scram-sha-256`은 구버전 PostgreSQL에서 사용할 수 없으므로, 스크립트는 버전에 따라 최종 `pg_hba.conf` auth method를 선택한다. v0.1에서는 구버전 호환성을 위해 `md5`를 1차 호환 경로로 반드시 지원한다.

```text
server_version_num < 100000:
  md5 사용

server_version_num >= 100000:
  서버가 scram-sha-256을 지원하고 운영자가 선택하면 scram-sha-256 사용
  그렇지 않으면 기존 서버 정책에 맞춰 md5 사용
```

auth method 선택 원칙:

```text
1. 기존 pg_hba.conf의 같은 유형 라인이 md5이면 md5 유지
2. PostgreSQL 10+이고 기존 정책이 scram-sha-256이면 scram-sha-256 사용
3. PostgreSQL 8~9이면 md5 사용
4. password 또는 trust를 최종 read-only 라인으로 남기지 않음
```

최종 라인 예시:

PostgreSQL 8~9 호환:

```conf
# Open Drug Bridge - Eghis read-only API
host    postgres    eghis_drug_ro    192.168.0.23/32    md5
```

PostgreSQL 10+에서 scram 사용 가능 시:

```conf
# Open Drug Bridge - Eghis read-only API
host    postgres    eghis_drug_ro    192.168.0.23/32    scram-sha-256
```

주의:

```text
md5는 최신 기준에서 scram-sha-256보다 약하지만, PostgreSQL 8~9 호환을 위해 필요하다.
따라서 md5를 사용할 경우 반드시 클라이언트 IP를 /32로 제한하고, 원내망/VPN 밖으로 노출하지 않는다.
```

구버전 호환을 위해 v0.1 서버 스크립트는 기본적으로 다음을 지원한다.

- `md5`
- `scram-sha-256`

지원하지 않는다.

- LDAP/Kerberos/SSPI 자동 설정
- replication 설정
- SSL 인증서 자동 발급

### 설치 경로 탐색

PostgreSQL 설치 경로가 표준 위치가 아닐 수 있으므로 자동 탐색과 수동 입력을 모두 지원한다.

탐색 후보:

```text
C:\Program Files\PostgreSQL\*\bin\psql.exe
C:\Program Files (x86)\PostgreSQL\*\bin\psql.exe
서비스 ImagePath에 포함된 postgres.exe 경로
Windows Registry의 PostgreSQL 설치 정보
PATH에 등록된 psql.exe
```

`pg_hba.conf` 후보:

```text
서비스 ImagePath의 -D <data_dir>
postgresql.conf가 있는 data directory
data\pg_hba.conf
사용자 수동 입력 경로
```

스크립트는 자동 탐색 결과를 바로 적용하지 않고, 사용자에게 확인을 받는다.

예:

```text
PostgreSQL service: postgresql-9.6
postgres.exe: C:\Program Files\PostgreSQL\9.6\bin\postgres.exe
psql.exe: C:\Program Files\PostgreSQL\9.6\bin\psql.exe
data directory: D:\Eghis\PostgreSQL\data
pg_hba.conf: D:\Eghis\PostgreSQL\data\pg_hba.conf

이 인스턴스에 읽기 전용 계정을 생성하시겠습니까? [yes/no]
```

### reload와 restart 정책

가능하면 `restart`보다 `reload`를 우선한다.

우선순위:

1. `pg_ctl reload -D <data_dir>`
2. `SELECT pg_reload_conf();`
3. Windows service restart

단, 임시 trust 추가 후 접속이 안 되면 restart가 필요할 수 있다. 이 경우 스크립트는 다음 경고를 표시한다.

```text
PostgreSQL 서비스를 재시작해야 할 수 있습니다.
재시작 중 EMR 접속이 일시적으로 끊길 수 있습니다.
진료시간 외 또는 서버 관리자 승인 후 진행하세요.
```

restart는 사용자가 명시적으로 확인한 경우에만 수행한다.

### SQL 호환성

구버전 호환을 위해 SQL은 가능한 단순하게 작성한다.

계정 생성은 버전에 따라 분기한다.

현대 버전:

```sql
CREATE ROLE eghis_drug_ro LOGIN PASSWORD :'password';
```

구버전 fallback:

```sql
CREATE USER eghis_drug_ro WITH PASSWORD '...';
```

권한 부여:

```sql
GRANT CONNECT ON DATABASE <db> TO eghis_drug_ro;
GRANT USAGE ON SCHEMA public TO eghis_drug_ro;
GRANT SELECT ON TABLE ... TO eghis_drug_ro;
```

PostgreSQL 8.x에서 일부 문법이 다를 수 있으므로, 실제 구현 시 최소 지원 버전을 먼저 실서버/샘플 환경에서 확인한다. `GRANT SELECT ON TABLE`이 실패하면 테이블별 `GRANT SELECT ON <table>`로 fallback한다.

### dry-run 모드

bootstrap 스크립트에는 반드시 dry-run 모드를 둔다.

dry-run에서 수행:

- PostgreSQL 서비스 탐색
- `psql.exe` 탐색
- `pg_hba.conf` 탐색
- PostgreSQL 버전 확인
- 적용 예정 `pg_hba.conf` 변경 라인 표시
- 적용 예정 SQL 표시
- 필요한 테이블 존재 여부 확인

dry-run에서 수행하지 않음:

- 파일 수정
- 서비스 reload/restart
- 계정 생성
- 권한 변경

### 백업과 복구

`pg_hba.conf` 수정 전 백업은 필수다.

백업 파일명:

```text
pg_hba.conf.opendrugbridge.bak.YYYYMMDD_HHMMSS
```

스크립트는 작업 종료 시 다음을 확인한다.

- 최종 `pg_hba.conf`에 임시 trust 라인이 없는지
- 최종 read-only 라인이 있는지
- PostgreSQL reload/restart가 성공했는지
- read-only 계정으로 SELECT가 가능한지
- read-only 계정으로 INSERT/UPDATE/DELETE가 실패하는지

실패 시:

1. 백업 파일로 `pg_hba.conf` 복구 시도
2. reload/restart 시도
3. 복구 결과 출력
4. 수동 복구 명령 출력

수동 복구 안내 예:

```powershell
Copy-Item "D:\Eghis\PostgreSQL\data\pg_hba.conf.opendrugbridge.bak.20260601_130000" "D:\Eghis\PostgreSQL\data\pg_hba.conf" -Force
Restart-Service "postgresql-9.6"
```

### 사용자 확인 단계

비개발자 운영자도 위험을 이해할 수 있게 확인 문구를 단계별로 둔다.

필수 확인:

1. 서버 인스턴스 선택 확인
2. `pg_hba.conf` 백업 경로 확인
3. 임시 local-only trust 사용 확인
4. 서비스 reload/restart 가능 시간 확인
5. 최종 클라이언트 IP 확인

confirmation phrase:

```text
로컬 임시 인증 후 복구
```

### 구현 우선순위

부트스트랩 스크립트는 앱 본체 개발 후반부에 구현한다. 먼저 문서와 dry-run 설계를 확정하고, 실제 수정 기능은 충분히 테스트한 뒤 제공한다.

우선 구현 순서:

1. PostgreSQL 탐색-only 스크립트
2. dry-run 스크립트
3. 관리자 계정이 있는 경우의 create 스크립트
4. bootstrap 스크립트
5. 실패 복구 테스트
6. 구버전 PostgreSQL 검증

#### pg_hba.conf 변경 정책

최종적으로 남길 수 있는 라인은 지정 클라이언트 IP의 읽기 전용 사용자만 허용하는 라인이다.

예:

```conf
# Open Drug Bridge - Eghis read-only API
host    postgres    eghis_drug_ro    192.168.0.23/32    scram-sha-256
```

허용하지 않는 설정:

```conf
host    all    all          0.0.0.0/0    trust
host    all    postgres     0.0.0.0/0    trust
host    all    all          0.0.0.0/0    md5
```

넓은 대역 허용이 필요하면 README에 수동 설정으로만 설명하고, 스크립트 기본 기능에는 넣지 않는다.

#### 백업 기능 보안 계획

향후 백업 기능은 기본적으로 약품 데이터 export backup만 앱에서 제공한다.

앱 내 backup export 기본 정책:

- 기본 저장 위치: 사용자 선택
- 자동 저장을 켤 때 경고 표시
- 파일명에 병원명/환자정보 포함 금지
- 파일에는 환자 개인정보 미포함
- XLSX/JSON/CSV export에는 약품 집계 데이터만 포함
- 전체 DB dump는 앱 UI에서 제공하지 않음

서버 관리자용 backup script를 추가할 경우:

- `scripts/eghis/maintenance/` 아래에 분리
- 관리자 권한 필요 명시
- 백업 파일 경로와 보관 책임 경고
- 가능하면 암호화 옵션 제공
- 기본값은 약품 관련 테이블만 dump
- 전체 DB dump는 별도 고급 옵션으로 분리

#### 업데이트와 배포 보안

v0.1은 자동 업데이트를 넣지 않는다.

이유:

- 코드 서명, 업데이트 서버, 무결성 검증 없이 자동 업데이트를 넣으면 공급망 위험이 커진다.

배포 원칙:

- GitHub release zip
- release checksum 제공
- 가능하면 Windows code signing 추후 적용
- release note에 DB schema 변경, 권한 변경 필요 여부 명시

#### 사용자 UX 안전 문구

UI 문구는 전문 용어보다 행동 중심으로 작성한다.

예:

```text
안전함: 이 계정은 읽기 전용입니다.
주의: 이 계정은 약품 조회에 필요한 것보다 많은 권한을 가지고 있습니다.
위험: 이 계정은 PostgreSQL 관리자 계정입니다. 앱에 저장할 수 없습니다.
위험: 현재 Web UI가 다른 PC에서도 접속 가능하도록 설정되어 있습니다.
위험: 인터넷 포트포워딩으로 이 앱을 공개하면 DB 정보와 약품 재고 정보가 유출될 수 있습니다.
권장: 원격 접속은 Tailscale/WireGuard 같은 VPN을 사용하세요.
권장: 원내망에서만 사용하고 Windows 방화벽으로 허용 IP를 제한하세요.
```

#### 보안 테스트 체크리스트

배포 전 다음 테스트를 통과해야 한다.

- superuser 계정 저장 차단
- 읽기 전용 계정 저장 허용
- password가 config/log에 평문으로 남지 않음
- `127.0.0.1` 외부에서 기본 접속 불가
- `0.0.0.0` 모드에서 token 없는 API 접근 실패
- `allowed_cidrs` 설정 시 허용 대역 외 접근 차단
- Host header 변조 요청 차단
- setup/save API token 없이 실패
- 환자명/수진번호가 API 응답에 포함되지 않음
- bootstrap 실패 시 pg_hba 백업 복구
- bootstrap 성공 후 임시 trust 라인 제거 확인
- read-only 계정으로 `INSERT/UPDATE/DELETE` 실패 확인
- XLSX export에 환자 식별정보 없음

## 6. 실행 흐름

### 최초 실행

1. 사용자가 `drug-storage-bridge.exe` 실행
2. 앱이 `127.0.0.1:3987`에서 HTTP 서버 시작
3. 기본 브라우저로 `http://127.0.0.1:3987/ui` 오픈
4. 설정 파일이 없으면 setup 화면 표시
5. 사용자가 PostgreSQL 읽기 계정 정보 입력
6. 연결 테스트
7. 성공 시 설정 저장
8. 약품 Web UI 사용 가능

### 재실행

1. 설정 파일 로드
2. Windows credential에서 비밀번호 복호화/조회
3. DB 연결 테스트
4. 성공 시 dashboard 표시
5. 실패 시 setup 화면으로 이동

## 7. 포트와 URL

기본 listen:

```text
127.0.0.1:3987
```

기본 URL:

```text
GET http://127.0.0.1:3987/ui
```

LAN 공유를 허용하려면 설정에서 명시적으로 host를 변경하게 한다.

```yaml
server:
  host: 0.0.0.0
  port: 3987
```

기본값은 로컬 PC 전용이어야 한다.

## 8. API 설계

### 시스템

```text
GET  /health
GET  /version
GET  /ui
```

### 설정

```text
GET  /api/setup/status
POST /api/setup/test-connection
POST /api/setup/save
POST /api/setup/reset
```

`POST /api/setup/test-connection` 요청 예:

```json
{
  "adapter": "eghis",
  "host": "192.168.0.10",
  "port": 5432,
  "database": "postgres",
  "user": "eghis_drug_ro",
  "password": "secret",
  "sslmode": "disable"
}
```

### 약품

```text
GET /api/drugs/search?q=
GET /api/drugs/{code}
GET /api/drugs/{code}/stock
GET /api/drugs/{code}/movements?from=YYYYMMDD&to=YYYYMMDD
```

### 처방량/주문량

```text
GET /api/usage?from=YYYYMMDD&to=YYYYMMDD
GET /api/inventory/order-plan?from=YYYYMMDD&to=YYYYMMDD&target_days=45
GET /api/inventory/order-plan.xlsx?from=YYYYMMDD&to=YYYYMMDD&target_days=45
```

## 9. Web UI 설계

### 화면

1. Setup
2. Dashboard
3. Drug Search
4. Inventory
5. Order Plan
6. Settings

### Setup 화면

입력 필드:

- EMR adapter: v0.1에서는 `eghis` 고정
- DB host
- DB port
- DB name
- DB user
- DB password
- SSL mode

버튼:

- 연결 테스트
- 저장

### Order Plan 화면

입력:

- 처방 시작일
- 처방 종료일
- 목표 비축일
- 일반약/향정약/전체 필터

표시:

- 주문 필요 품목 수
- 긴급 품목 수
- 권장 주문량 합계
- 평균 재고일수
- 주문 필요량 표
- 전체 상세 표
- XLSX 다운로드

중요 컬럼:

- 긴급도
- 구분
- 성분명
- 용량
- 대표 약품명
- 현재재고
- 재고일수
- 목표재고
- 부족량
- 권장주문량
- 재고출처

## 10. Go 프로젝트 구조

```text
open-drug-bridge/
  cmd/
    drug-storage-bridge/
      main.go
  internal/
    app/
    api/
    config/
    credential/
    drug/
    inventory/
    export/
    server/
    web/
    adapters/
      eghis/
        schema.go
        drugs.go
        stock_general.go
        stock_nims.go
        usage.go
  web/
    index.html
    assets/
      app.js
      styles.css
  scripts/
    eghis/
      bootstrap_eghis_drug_readonly.ps1
      create_eghis_drug_readonly.ps1
  doc/
    development_plan.md
  config.example.yaml
  README.md
  LICENSE
```

## 11. Adapter 인터페이스

EMR별 구현은 adapter로 격리한다.

Go interface 초안:

```go
type DrugAdapter interface {
    Name() string
    TestConnection(ctx context.Context) error
    SearchDrugs(ctx context.Context, query string) ([]Drug, error)
    GetDrug(ctx context.Context, code string) (Drug, error)
    GetStock(ctx context.Context, code string) (StockBalance, error)
    GetUsage(ctx context.Context, from, to string) ([]DrugUsage, error)
    BuildOrderPlan(ctx context.Context, from, to string, targetDays int) ([]OrderPlanRow, error)
}
```

v0.1 구현:

```text
internal/adapters/eghis
```

향후:

```text
internal/adapters/other_emr
```

## 12. Eghis DB 읽기 권한 대상

v0.1에서 필요한 테이블 후보:

```text
h0_mst_drug
h0drug_stock
h1opdin
h2opd_doct_ord
h8_nims_buy
h8_nims_buy_lines
h8_nims_medi
h8_nims_medi_lines
h8_nims_exp
h8_nims_exp_lines
h8_nims_send
h8_nims_send_lines
```

추가 테이블은 실제 구현 중 쿼리 검증 후 최소 권한 원칙에 따라 확정한다.

## 13. Eghis 재고 계산 로직

### 일반약

```text
현재재고 = 누적 입고 - 반품/폐기 - 누적 원내 사용
```

입고/반품/폐기:

```text
h0drug_stock
```

원내 사용:

```text
h2opd_doct_ord.qty * h2opd_doct_ord.days
```

조건:

- `inout_gb = 'I'`
- 첫 입고일 이후 처방만 사용량에 포함
- 외래 조제 마감된 건만 포함
- 약품코드는 `ord_cd`, `medfee_cd`, `user_cd` fallback

### 향정/마약류

NIMS 보고 테이블 기준으로 계산한다.

```text
현재재고 = 구입보고 - 투약보고 - 폐기보고 - 양도보고
```

수량 공식:

```text
구입: min_distb_qy * prd_tot_pce_qy + pce_qy
투약: pce_qy
폐기: min_distb_qy * prd_tot_pce_qy + pce_qy
양도: min_distb_qy * prd_tot_pce_qy + pce_qy
```

보고 헤더 조건:

```text
sts_cd = '20'
result_cd = '0000'
```

정정/취소 처리:

- `rpt_ty_cd = '0'`: 신규보고. 단, 성공한 정정/취소 보고가 참조한 원 보고는 제외
- `rpt_ty_cd = '1'`: 취소보고. 이동량으로 반영하지 않고 참조 원 보고만 제외
- `rpt_ty_cd = '2'`: 정정보고. 원 보고를 대체하는 이동량으로 반영

제품코드와 약품코드는 `h8_nims_medi_lines.user_cd`와 `prduct_cd`로 매핑한다.

## 14. 서버 설정 스크립트 계획

스크립트는 EMR adapter별 폴더에 둔다.

```text
scripts/
  eghis/
    create_eghis_drug_readonly.ps1
    bootstrap_eghis_drug_readonly.ps1
```

### create 스크립트

DB superuser 계정/비밀번호를 아는 경우 사용한다.

역할:

1. PostgreSQL 접속
2. 읽기 전용 사용자 생성
3. 약품 관련 테이블에만 SELECT 부여
4. `pg_hba.conf`에 지정 클라이언트 IP 허용
5. PostgreSQL reload
6. 접속 테스트

최종 `pg_hba.conf` 예:

```conf
# Eghis Drug Bridge read-only API
host    postgres    eghis_drug_ro    192.168.0.23/32    scram-sha-256
```

### bootstrap 스크립트

DB superuser 비밀번호는 모르지만 서버 OS 관리자 권한이 있는 경우 사용한다.

역할:

1. PostgreSQL 서비스명 탐색 또는 입력
2. `psql.exe` 탐색
3. `pg_hba.conf` 탐색
4. `pg_hba.conf` 백업
5. 로컬 postgres 접속만 임시 trust 추가
6. PostgreSQL restart 또는 reload
7. 로컬에서 `postgres`로 접속
8. 읽기 전용 사용자 생성
9. 필요한 SELECT 권한 부여
10. 클라이언트 IP용 `scram-sha-256` 라인 추가
11. 임시 trust 라인 제거
12. PostgreSQL restart 또는 reload
13. 읽기 전용 사용자 접속 테스트
14. 실패 시 백업 복구

임시 trust는 로컬에만 허용한다.

```conf
# TEMP Open Drug Bridge bootstrap - local only
host    all    postgres    127.0.0.1/32    trust
host    all    postgres    ::1/128         trust
```

금지:

```conf
host    all    all    0.0.0.0/0    trust
```

bootstrap 스크립트는 작업 종료 시 임시 trust 라인이 남아 있으면 실패로 처리해야 한다.

## 15. 설정 파일 위치

Windows 기본:

```text
%APPDATA%\OpenDrugBridge\config.yaml
```

로그:

```text
%LOCALAPPDATA%\OpenDrugBridge\logs\
```

## 16. XLSX export

Order Plan export는 XLSX로 제공한다.

시트:

```text
요약
주문필요
전체상세
```

구현:

```text
github.com/xuri/excelize/v2
```

## 17. 향후 백업 기능 계획

백업 기능은 v0.1 범위에는 포함하지 않는다. 다만 향후 확장을 고려해 권한 모델을 앱 런타임 권한과 서버 유지보수 권한으로 분리한다.

### 백업 유형

| 유형 | 설명 | 읽기 전용 계정으로 가능 여부 | 구현 위치 |
| --- | --- | --- | --- |
| 약품 데이터 export backup | 약품마스터, 재고, NIMS 보고, 처방량 집계 등을 XLSX/CSV/JSON으로 저장 | 가능 | 앱 기능 |
| 약품 관련 테이블 logical dump | 약품 관련 테이블만 `pg_dump`로 백업 | 대체로 가능, 환경별 검증 필요 | 별도 서버 스크립트 |
| 전체 DB logical dump | 전체 PostgreSQL DB를 `pg_dump`로 백업 | 부족할 수 있음 | 별도 관리자 스크립트 |
| 물리 백업 | `pg_basebackup`, WAL archiving, data directory copy 등 | 불가능 | 앱 외부 운영 영역 |

### 권한 모델

권한은 최소 4개 범주로 나눈다.

```text
1. app_readonly
   - 기본 앱 실행용
   - 약품 조회, 재고 계산, 주문량 계산
   - SELECT only

2. backup_exporter
   - 선택 기능
   - 약품 관련 테이블 export 또는 제한적 logical dump
   - SELECT + 필요한 schema 접근

3. admin_bootstrap
   - 서버 초기 설정 스크립트에서만 임시 사용
   - read-only 계정 생성
   - pg_hba.conf 수정
   - 앱 런타임에서는 사용하지 않음

4. physical_backup
   - 향후 고급 운영 기능
   - OS 관리자/replication 권한 필요
   - 앱 본체와 분리된 서버 운영 스크립트로만 제공
```

### 설계 원칙

- 앱 본체는 관리자 DB 계정이나 PostgreSQL superuser 비밀번호를 저장하지 않는다.
- 앱 본체는 `pg_hba.conf`를 수정하지 않는다.
- 전체 DB 백업/복구는 앱 기능이 아니라 서버 관리자용 maintenance script/plugin으로 분리한다.
- 약품 데이터 export backup은 읽기 전용 계정으로 구현 가능한 범위에서만 제공한다.
- 백업 파일 저장 경로, 접근 권한, 암호화, 보관 기간은 서버 관리자/소유자의 책임으로 명시한다.

## 18. 개발 단계

### Phase 1. 문서와 스캐폴딩

- 개발 계획 문서 작성
- Go module 생성
- 기본 폴더 구조 생성
- README 초안
- config example 작성

### Phase 2. 서버와 설정 UI

- HTTP server
- embedded Web UI
- setup status API
- DB 연결 테스트 API
- config 저장
- Windows credential 저장

### Phase 3. Eghis adapter

- 약품 검색
- 약품 상세
- 일반약 재고 계산
- 향정/마약류 NIMS 재고 계산
- 처방량 집계
- 주문 필요량 계산

### Phase 4. Export와 Web UI 정리

- 주문 계획 UI
- XLSX export
- 설정 초기화 UI
- 오류 메시지 정리

### Phase 5. Windows 서버 스크립트

- create script
- bootstrap script
- pg_hba 백업/복구
- reload/restart
- 연결 테스트

### Phase 6. 배포

- Windows exe build
- release zip
- README 설치 가이드
- GitHub Actions build

## 19. v0.1 완료 기준

- Windows에서 단일 exe 실행 가능
- 최초 setup Web UI 동작
- DB 접속 설정 저장 및 재사용 가능
- 읽기 전용 계정으로만 동작
- 약품 검색 가능
- 일반약 재고 계산 가능
- 향정/마약류 재고 계산 가능
- 주문 필요량 계산 가능
- XLSX export 가능
- Eghis 서버 bootstrap 스크립트 동작
- 임시 trust 인증이 작업 후 자동 제거됨
- README만 보고 설치 가능한 수준

## 20. 공개 전 체크리스트

- 환자 개인정보가 API 응답에 포함되지 않는지 확인
- 설정 파일에 비밀번호 평문이 없는지 확인
- 로그에 DSN/password가 남지 않는지 확인
- `pg_hba.conf` bootstrap 후 임시 trust 라인이 제거되는지 확인
- 읽기 전용 계정으로 INSERT/UPDATE/DELETE가 실패하는지 확인
- README에 서버 관리자/소유자 보안 책임 고지가 들어갔는지 확인
- 백업 기능이 추가될 경우 백업 파일 보관 책임과 민감정보 위험이 고지되었는지 확인
- Windows Defender에서 실행 파일 경고가 과도하지 않은지 확인
- GitHub release에 sample config와 scripts 포함 확인

