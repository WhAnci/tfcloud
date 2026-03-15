# tfcloud

Kubernetes 스타일 YAML 매니페스트를 Terraform HCL 파일로 변환하는 CLI 도구입니다. 선택적으로 `terraform` 명령어를 실행할 수도 있습니다.

## 개요

`tfcloud`는 익숙한 Kubernetes 매니페스트 형식으로 AWS 인프라를 정의하고, 이를 깔끔하고 구조화된 Terraform 코드로 자동 변환해줍니다.

### 주요 특징

- Kubernetes 스타일 YAML로 인프라 정의
- 유효한 Terraform HCL 자동 생성
- `terraform init/plan/apply/destroy` 명령어 래핑
- 새로운 리소스 타입을 쉽게 추가할 수 있는 확장 가능한 아키텍처

## 설치

### Go install

```bash
go install github.com/WhAnci/tfcloud@latest
```

### 소스에서 빌드

```bash
git clone https://github.com/WhAnci/tfcloud.git
cd tfcloud
go build -o tfcloud .
```

## 사용법

### YAML 매니페스트 작성

Kubernetes 스타일의 YAML 파일을 작성합니다:

```yaml
apiVersion: v1
kind: VPC
metadata:
  name: wh-vpc-01
spec:
  region: ap-northeast-2
  cidrBlock: 10.0.0.0/16
  ipv6CidrBlock: false
  public:
    - name: wsi-public-subnet-a
      cidr: 10.0.1.0/24
      zone: a
    - name: wsi-public-subnet-c
      cidr: 10.0.2.0/24
      zone: c
  private:
    - name: wsi-private-subnet-a
      cidr: 10.0.100.0/24
      zone: a
    - name: wsi-private-subnet-c
      cidr: 10.0.200.0/24
      zone: c
  route:
    internetGateway:
      enabled: true
      name: wsi-igw
    natGateway:
      strategy: regional
      name: wsi-ngw
    publicRouteTablePerAz: false
    privateRouteTablePerAz: true
  dns:
    hostnames: true
    resolution: true
  tags:
    Project: wh-skills
    Env: dev
```

### CLI 명령어

#### `generate` - Terraform 파일 생성

YAML 매니페스트를 파싱하여 `.tf` 파일을 생성합니다.

```bash
# 기본 출력 디렉토리 (./output/)에 생성
tfcloud generate -f examples/vpc.yaml

# 커스텀 출력 디렉토리 지정
tfcloud generate -f examples/vpc.yaml -o ./terraform/
```

#### `plan` - 생성 + terraform plan

Terraform 파일을 생성한 후 `terraform init`과 `terraform plan`을 실행합니다.

```bash
tfcloud plan -f examples/vpc.yaml -o ./terraform/
```

#### `apply` - 생성 + terraform apply

Terraform 파일을 생성한 후 `terraform init`과 `terraform apply`를 실행합니다.

```bash
# 대화형 승인
tfcloud apply -f examples/vpc.yaml

# 자동 승인
tfcloud apply -f examples/vpc.yaml --auto-approve
```

#### `destroy` - terraform destroy

출력 디렉토리에서 `terraform destroy`를 실행합니다.

```bash
# 대화형 승인
tfcloud destroy -o ./output/

# 자동 승인
tfcloud destroy -o ./output/ --auto-approve
```

## NAT Gateway 전략

YAML의 `spec.route.natGateway.strategy` 필드로 NAT Gateway 배포 전략을 선택할 수 있습니다:

| 전략 | 설명 |
|------|------|
| `regional` / `single` | 첫 번째 퍼블릭 서브넷 AZ에 NAT 1개 생성. 모든 프라이빗 서브넷이 공유 |
| `per-az` | AZ별 NAT 1개씩 생성. 각 프라이빗 서브넷은 해당 AZ의 NAT 사용 |
| `none` | NAT Gateway 미생성 |

### `per-az` 전략의 이름 지정

```yaml
# 문자열: 접두사로 사용, AZ 접미사 자동 추가 (예: wsi-ngw-a, wsi-ngw-c)
natGateway:
  strategy: per-az
  name: wsi-ngw

# 리스트: AZ별 명시적 이름 지정
natGateway:
  strategy: per-az
  name:
    - a: ngw-az-a
    - c: ngw-az-c
```

## 생성되는 Terraform 리소스

VPC 매니페스트에서 생성되는 리소스 목록:

- `aws_vpc` - VPC
- `aws_subnet` - 퍼블릭/프라이빗 서브넷
- `aws_internet_gateway` - 인터넷 게이트웨이
- `aws_eip` - NAT Gateway용 탄력적 IP
- `aws_nat_gateway` - NAT 게이트웨이
- `aws_route_table` - 라우트 테이블
- `aws_route` - 라우트 규칙
- `aws_route_table_association` - 서브넷-라우트 테이블 연결

## 출력 예시

`examples/vpc.yaml`에서 생성되는 Terraform 코드의 일부:

```hcl
terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}

provider "aws" {
  region = "ap-northeast-2"
}

resource "aws_vpc" "wh_vpc_01" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true
  tags = {
    Env     = "dev"
    Name    = "wh-vpc-01"
    Project = "wh-skills"
  }
}

resource "aws_subnet" "wsi_public_subnet_a" {
  vpc_id                  = aws_vpc.wh_vpc_01.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = "ap-northeast-2a"
  map_public_ip_on_launch = true
  tags = {
    Env     = "dev"
    Name    = "wsi-public-subnet-a"
    Project = "wh-skills"
  }
}
```

## 새로운 리소스 타입 추가 방법

1. `pkg/parser/parser.go`에 새로운 Spec 구조체를 정의합니다:

```go
type EKSSpec struct {
    Region      string `yaml:"region"`
    ClusterName string `yaml:"clusterName"`
    Version     string `yaml:"version"`
    // ...
}
```

2. `Parse` 함수의 `switch` 문에 새 Kind를 추가합니다:

```go
case "EKS":
    var spec EKSSpec
    if err := raw.Spec.Decode(&spec); err != nil {
        return nil, fmt.Errorf("failed to parse EKS spec: %w", err)
    }
    manifest.Spec = &spec
```

3. `pkg/generator/` 디렉토리에 새 생성기 파일을 만듭니다 (예: `eks.go`):

```go
type EKSGenerator struct{}

func (g *EKSGenerator) Kind() string     { return "EKS" }
func (g *EKSGenerator) Filename() string { return "eks.tf" }

func (g *EKSGenerator) Generate(manifest *parser.Manifest) ([]byte, error) {
    // HCL 생성 로직
}

func init() {
    Register(&EKSGenerator{})
}
```

4. `pkg/generator/generator.go`의 `init()`에서 새 생성기가 자동으로 등록됩니다 (각 파일의 `init()` 사용).

## 필수 조건

- Go 1.21 이상
- Terraform CLI (`apply`, `plan`, `destroy` 명령어 사용 시)

## 라이선스

MIT
