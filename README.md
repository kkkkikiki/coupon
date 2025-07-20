# Coupon Service

고성능 쿠폰 발급 시스템입니다. 이 서비스는 다음과 같은 기능을 제공합니다:
- 제한된 수량의 쿠폰 발급 (선착순)
- 초당 500-1,000건의 트래픽 처리
- 정확한 쿠폰 수량 관리 (과다 발급 방지)
- 지정된 시간에 자동 시작
- 데이터 일관성 보장
- 고유한 쿠폰 코드 생성 (한글 + 숫자, 최대 10자)

## 🚀 시작하기

### 사전 요구사항
- Docker 및 Docker Compose
- Go 1.21 이상 (로컬 개발용)
- make (선택사항, 편의 스크립트 사용 시)

### 환경 설정

1. 환경 변수 파일 복사 및 설정:
   ```bash
   cp .env.example .env
   ```
   필요한 경우 `.env` 파일을 수정합니다.

2. 의존성 설치 (로컬 개발 시):
   ```bash
   go mod download
   ```

## 🏃 서버 실행 방법

### Docker Compose를 사용한 실행 (권장)

```bash
# 모든 서비스 시작 (PostgreSQL + Coupon Service + Nginx)
docker-compose up -d

# 서비스 상태 확인
docker-compose ps

# 로그 확인
docker-compose logs -f

# 서비스 중지
docker-compose down
```

### 로컬에서 실행 (개발용)

1. PostgreSQL 서버가 실행 중인지 확인하세요.
2. 서버 실행:
   ```bash
   go run cmd/server/main.go
   ```

## 🧪 테스트 실행

### 종합 테스트 스크립트

모든 요구사항을 검증하는 종합 테스트를 실행합니다:

```bash
# 실행 권한 부여 (처음 한 번만)
chmod +x test-all-requirements.sh

# 테스트 실행
./test-all-requirements.sh
```

### 프로젝트 구조
```
.
├── Dockerfile
├── Makefile
├── buf.gen.yaml
├── buf.yaml
├── cmd
│   ├── main.go
│   └── perf-client
│       └── main.go
├── docker-compose.yml
├── gen
│   └── coupon
│       └── v1
│           ├── coupon.pb.go
│           └── couponv1connect
│               └── coupon.connect.go
├── go.mod
├── go.sum
├── internal
│   ├── config
│   │   └── config.go
│   ├── database
│   │   └── database.go
│   ├── metrics
│   │   └── metrics.go
│   ├── model
│   │   └── campaign.go
│   ├── repository
│   │   ├── campaign_repository.go
│   │   └── coupon_repository.go
│   └── service
│       └── coupon_service.go
├── nginx
│   ├── conf.d
│   │   └── coupon.conf
│   └── nginx.conf
├── proto
│   └── coupon
│       └── v1
│           └── coupon.proto
├── scripts
│   └── init.sql
└── test-all-requirements.sh
```

### Makefile 명령어

프로젝트 관리를 위한 유용한 Make 명령어들입니다:

```bash
# 모든 명령어 확인
make help

# 애플리케이션 빌드
make build

# 서버 실행
make run

# 테스트 실행
make test

# 프로토버프 코드 생성
make proto-gen

# 개발 환경 설정
make setup

# Docker 서비스 시작 (PostgreSQL, Redis 등)
make docker-up

# Docker 서비스 중지
make docker-down

# 빌드 아티팩트 정리
make clean
```

### 코드 생성

프로토콜 버퍼 파일을 수정한 후 다음 명령어로 코드를 생성합니다:

```bash
make proto-gen
```