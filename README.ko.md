# EMR Drug Manager 한국어 설치 안내

이 문서는 개발을 모르는 사용자도 Windows 서버에서 앱을 설치하고, eGHIS PostgreSQL 읽기 전용 계정을 만든 뒤, 재고 조회와 처방량 통계를 사용할 수 있도록 순서대로 설명합니다.

## 무엇을 하는 프로그램인가요?

EMR Drug Manager는 eGHIS PostgreSQL 데이터베이스를 읽어서 약품 재고, 처방량 통계, 주문 계획을 보여주는 Windows용 로컬 프로그램입니다.

- 데이터베이스를 수정하지 않습니다.
- 약품 관련 테이블만 읽습니다.
- 환자 식별 정보를 조회하는 화면/API는 제공하지 않습니다.
- 기본 주소는 `127.0.0.1:3987`이라 같은 컴퓨터에서만 접속됩니다.

## 설치 전 준비물

필요한 것:

- eGHIS PostgreSQL이 설치된 Windows 서버 컴퓨터
- Windows 관리자 권한
- 이 저장소의 파일 전체
- `drug-storage-bridge.exe`

권장:

- PostgreSQL 관리자 계정 비밀번호를 알고 있으면 더 간단합니다.
- 모르면 `bootstrap_eghis_drug_readonly.ps1` 스크립트로 로컬 임시 인증을 사용해 읽기 전용 계정을 만들 수 있습니다.

## 1. 파일 받기

GitHub에서 초록색 `Code` 버튼을 누른 뒤 `Download ZIP`을 선택합니다.

압축을 예를 들어 아래 위치에 풉니다.

```text
C:\EMRDrugManager
```

이 폴더 안에 다음 파일이 있어야 합니다.

```text
drug-storage-bridge.exe
scripts\eghis\bootstrap_eghis_drug_readonly.ps1
scripts\eghis\create_eghis_drug_readonly.ps1
```

## 2. 읽기 전용 DB 계정 만들기

앱에는 PostgreSQL 관리자 계정을 넣지 마세요. 앱 전용 읽기 전용 계정을 만들어 사용해야 합니다.

기본값은 다음과 같습니다.

```text
DB 이름: postgres
DB 주소: 127.0.0.1
DB 포트: 5432
```

### 방법 A. PostgreSQL 관리자 비밀번호를 아는 경우

PowerShell을 관리자 권한으로 엽니다.

```powershell
cd C:\EMRDrugManager
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\scripts\eghis\create_eghis_drug_readonly.ps1 -DryRun
```

문제가 없어 보이면 실제 실행합니다.

```powershell
.\scripts\eghis\create_eghis_drug_readonly.ps1
```

진행 중 입력할 것:

- 새 읽기 전용 사용자 이름
- 새 읽기 전용 사용자 비밀번호
- PostgreSQL 관리자 계정 비밀번호
- `pg_hba.conf`가 여러 개 나오면 eGHIS가 사용하는 PostgreSQL 버전의 번호

보통 사용자 이름은 아래처럼 두면 됩니다.

```text
eghis_drug_ro
```

### 방법 B. PostgreSQL 관리자 비밀번호를 모르는 경우

서버 관리자라면 bootstrap 스크립트를 사용할 수 있습니다. 이 스크립트는 로컬 컴퓨터에서만 임시로 PostgreSQL 관리자 접속을 허용한 뒤, 읽기 전용 계정을 만들고, 임시 설정을 되돌립니다.

PowerShell을 관리자 권한으로 엽니다.

```powershell
cd C:\EMRDrugManager
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\scripts\eghis\bootstrap_eghis_drug_readonly.ps1 -DryRun
```

문제가 없어 보이면 실제 실행합니다.

```powershell
.\scripts\eghis\bootstrap_eghis_drug_readonly.ps1
```

진행 중 입력할 것:

- 새 읽기 전용 사용자 이름
- `pg_hba.conf` 선택 번호
- 새 읽기 전용 사용자 비밀번호
- 확인 문구: `로컬 임시 인증 후 복구`

`Multiple pg_hba.conf candidates found`가 나오면 eGHIS가 실제로 사용하는 PostgreSQL 버전을 선택하세요. 예를 들어 PostgreSQL 18을 사용 중이면 `18\data\pg_hba.conf` 쪽을 선택합니다.

스크립트가 PostgreSQL 서비스를 자동으로 찾지 못하면, 중간에 PostgreSQL을 수동으로 다시 시작하라고 안내할 수 있습니다. 이 경우 Windows 서비스 앱에서 PostgreSQL 서비스를 다시 시작한 뒤 Enter를 누르면 됩니다.

## 3. 앱 실행하기

일반 PowerShell 또는 파일 탐색기에서 실행합니다.

```powershell
cd C:\EMRDrugManager
.\drug-storage-bridge.exe
```

정상 실행되면 대략 아래처럼 표시됩니다.

```text
starting Drug Storage Bridge
addr=127.0.0.1:3987
```

브라우저에서 엽니다.

```text
http://127.0.0.1:3987/ui
```

처음 실행하면 DB 접속 정보를 입력합니다.

```text
DB 주소: 127.0.0.1
DB 포트: 5432
DB 이름: postgres
사용자: 위에서 만든 읽기 전용 사용자
비밀번호: 위에서 만든 읽기 전용 사용자 비밀번호
```

연결 테스트가 성공하면 저장합니다.

## 4. 사용하는 방법

### 재고 조회

전체 약품 재고를 조회합니다.

사용할 수 있는 기능:

- 현재 재고량 확인
- 약품코드, 약품명 확인
- XLSX 다운로드

### 처방량 통계

최근 기간 또는 직접 지정한 기간의 처방량을 조회합니다.

예:

- 최근 30일
- 최근 3개월
- 시작일과 종료일 직접 입력

사용할 수 있는 기능:

- 기간별 처방량 확인
- 동일성분용량 합산
- 원외약 제외
- 주사제 제외
- XLSX 다운로드

### 주문 계획

선택한 처방 기간을 기준으로 필요한 주문 수량을 계산합니다.

화면의 시작일과 종료일은 “처방기간”입니다.

주요 컬럼:

- 약품코드
- 약품명
- 기간처방량
- 비축필요량
- 현재재고
- 주문필요수량

사용할 수 있는 기능:

- 최근 3개월 같은 빠른 기간 선택
- 동일성분용량 합산 (필요할 때만 선택; 기본은 품목별 계산)
- 원외약 제외
- 주사제 제외
- 필요수량 100단위 올림 (기본 적용)
- XLSX 다운로드

## 5. 다른 컴퓨터에서 접속하려면

기본 설정은 서버 컴퓨터 자신만 접속할 수 있습니다.

다른 컴퓨터에서 접속해야 한다면 Tailscale 사용을 강력히 권장합니다. 공유기 포트포워딩 없이, 허가된 컴퓨터끼리만 사설 VPN처럼 연결할 수 있어 병원 내부 도구를 노출하지 않고 쓰기 좋습니다.

### 권장 방법: Tailscale

1. 서버 컴퓨터에 Tailscale을 설치합니다.

   공식 다운로드:

   ```text
   https://tailscale.com/download/windows
   ```

2. 서버 컴퓨터에서 Tailscale에 로그인합니다.

   설치 후 오른쪽 아래 작업표시줄의 Tailscale 아이콘을 눌러 로그인합니다. Google, Microsoft, GitHub 계정 등으로 로그인할 수 있습니다.

3. 접속할 다른 컴퓨터에도 Tailscale을 설치하고 같은 계정 또는 같은 조직으로 로그인합니다.

4. 서버 컴퓨터의 Tailscale IP를 확인합니다.

   Tailscale 아이콘을 누르면 `100.x.y.z` 형태의 주소가 보입니다. Tailscale은 기본적으로 `100.64.0.0/10` 대역의 사설 주소를 각 기기에 부여합니다.

5. 앱 설정 파일을 엽니다.

   설정 파일 위치는 보통 아래입니다.

```text
%APPDATA%\OpenDrugBridge\config.yaml
```

6. `server.host`를 `0.0.0.0`으로 바꿉니다.

```yaml
server:
  host: 0.0.0.0
  port: 3987
```

7. 앱을 다시 실행합니다.

```powershell
.\drug-storage-bridge.exe
```

8. 다른 컴퓨터에서 접속합니다.

아래 주소에서 `100.x.y.z` 부분을 서버 컴퓨터의 Tailscale IP로 바꿉니다.

```text
http://100.x.y.z:3987/ui
```

9. Windows 방화벽에서 차단되면 허용합니다.

Windows 방화벽 알림이 뜨면 개인 네트워크에서 허용합니다. 수동으로 방화벽을 설정한다면 TCP `3987` 포트를 Tailscale 대역인 `100.64.0.0/10`에서만 허용하는 것이 좋습니다.

### 선택: 접속 토큰 추가

Tailscale을 쓰더라도 여러 사람이 같은 tailnet에 들어올 수 있는 환경이면 앱 접속 토큰을 추가하는 것이 좋습니다.

서버에서 긴 임의 문자열을 하나 정하고 해시를 만듭니다.

```powershell
.\drug-storage-bridge.exe --hash-token "여기에-긴-비밀문자열"
```

출력된 해시를 설정 파일에 넣습니다.

```yaml
server:
  host: 0.0.0.0
  port: 3987
  access_token_required: true
  allowed_cidrs:
    - 100.64.0.0/10
  access_token_hash: "출력된_SHA256_해시"
```

원래 비밀문자열은 따로 안전하게 보관하고, 설정 파일에는 해시만 저장하세요.

### 같은 병원 내부망만 사용할 때

Tailscale 없이 같은 병원 내부망에서만 사용할 수도 있습니다. 이 경우에도 서버 설정은 아래처럼 바꿉니다.

```yaml
server:
  host: 0.0.0.0
  port: 3987
```

그리고 Windows 방화벽에서 접속할 PC의 내부 IP만 허용하세요.

외부 인터넷에 직접 공개하지 마세요.

하지 말아야 할 것:

- 공유기 포트포워딩
- 공인 IP 직접 노출
- 방화벽 전체 허용

원격 접속이 필요하면 Tailscale, ZeroTier, NetBird, 병원 VPN 같은 사설 네트워크를 권장합니다.

## 6. 자주 생기는 문제

### 이미 실행 중이라는 오류

아래 오류가 나오면 이미 앱이 켜져 있는 것입니다.

```text
bind: Only one usage of each socket address
```

이미 떠 있는 앱에 접속하세요.

```text
http://127.0.0.1:3987/ui
```

### DB 연결 실패

확인할 것:

- PostgreSQL 서비스가 실행 중인지
- DB 이름이 `postgres`인지
- 포트가 `5432`인지
- 읽기 전용 사용자 이름과 비밀번호가 맞는지
- `pg_hba.conf`에 해당 사용자의 접속 허용 줄이 들어갔는지

### 브라우저가 다운로드를 막는 경우

가능하면 앱을 아래 주소로 접속하세요.

```text
http://127.0.0.1:3987/ui
```

다른 컴퓨터에서 접속할 때는 HTTP 다운로드 정책 때문에 브라우저가 차단할 수 있습니다. 내부망/VPN 환경에서 사용하고, 브라우저의 다운로드 허용 설정을 확인하세요.

## 7. 보안 주의

- 앱에는 PostgreSQL 관리자 계정을 저장하지 마세요.
- 반드시 읽기 전용 계정을 사용하세요.
- 설정 파일이나 비밀번호 파일을 다른 사람에게 보내지 마세요.
- 병원/의원 개인정보보호 정책을 따르세요.
- 이 앱은 서버 보안을 대신 관리해주지 않습니다.

## 8. 개발자용 명령

개발자가 직접 빌드할 때만 필요합니다.

```powershell
go test ./...
go build -trimpath -ldflags="-s -w" -o dist\drug-storage-bridge.exe .\cmd\drug-storage-bridge
```
