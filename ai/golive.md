**Plan wdrożenia na live (Serverless + DynamoDB)**

**1. Przygotowanie schemy DynamoDB i migracji**
1. Zdefiniuj tabelę główną (single-table):
   - `Table: sqoush-app`
   - `PK` (string), `SK` (string)
   - GSI: `GSI1PK`, `GSI1SK` (np. lookup po emailu i listy meczów)
2. Spisz model danych (poniżej gotowy single-table).
3. Utwórz `db/migrations/` i wersjonuj migracje jako idempotentne pliki.
4. Dodaj prosty runner migracji (Go) z tabelą `sqoush-migrations`.
5. Każda zmiana schemy = nowa migracja (nigdy nie edytuj już wdrożonych).

**2. Wersjonowanie projektu**
1. SemVer: `MAJOR.MINOR.PATCH`.
2. Trzymaj wersję w `VERSION` oraz tagach Git (`v0.5.0`).
3. Każdy deploy produkcyjny = tag release.

**3. CI/CD (GitHub Actions)**
1. CI: `go test ./...`, `go build ./...`.
2. CD: build binarki, zip, deploy (AWS CLI/SAM/Terraform).
3. Sekrety: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `DYNAMODB_TABLE`.
4. Przed deployem wykonaj `migrate up`.

**4. Tagowanie i release**
1. Aktualizuj `VERSION` i `CHANGELOG.md`.
2. Taguj: `git tag vX.Y.Z`.
3. Push tag uruchamia deploy produkcyjny.

---

**Single-table (DynamoDB) pod obecne modele**

**Tabela:** `sqoush-app`
- `PK` (string)
- `SK` (string)
- `GSI1PK` (string)
- `GSI1SK` (string)

**Użytkownik**
- PK: `USER#{userId}`
- SK: `PROFILE`
- Atrybuty: `FirstName`, `LastName`, `Email`, `Phone`, `Role`, `Skill`, `AvatarURL`, `CreatedAt`
- GSI (email lookup):
  - `GSI1PK = EMAIL#{email}`
  - `GSI1SK = USER#{userId}`

**Liga**
- PK: `LEAGUE#{leagueId}`
- SK: `META`
- Atrybuty: `Name`, `Description`, `Location`, `OwnerID`, `AdminRoles`, `StartDate`, `EndDate`, `Status`, `SetsPerMatch`, `CreatedAt`

**Członkostwo w lidze (gracz)**
- PK: `LEAGUE#{leagueId}`
- SK: `PLAYER#{userId}`
- Atrybuty: `UserID`, `RoleInLeague=player`

**Admin ligi**
- PK: `LEAGUE#{leagueId}`
- SK: `ADMIN#{userId}`
- Atrybuty: `UserID`, `AdminRole` (admin-player/moderator)

**Mecz ligowy**
- PK: `LEAGUE#{leagueId}`
- SK: `MATCH#{matchId}`
- Atrybuty: `PlayerAID`, `PlayerBID`, `Sets`, `Status`, `ReportedBy`, `ConfirmedBy`, `CreatedAt`
- GSI (historia użytkownika):
  - `GSI1PK = USER#{userId}` (dla PlayerA/PlayerB duplikowane wpisy lub osobny index item)
  - `GSI1SK = MATCH#{createdAt}`

**Mecz towarzyski**
- PK: `FRIENDLY#{matchId}`
- SK: `META`
- Atrybuty: `PlayerAID`, `PlayerBID`, `Sets`, `Status`, `ReportedBy`, `ConfirmedBy`, `PlayedAt`, `CreatedAt`
- GSI (historia użytkownika):
  - `GSI1PK = USER#{userId}`
  - `GSI1SK = FRIENDLY#{playedAt}`

---

**Szablon migracji (JSON)**
`db/migrations/001_create_table.json`
```json
{
  "TableName": "sqoush-app",
  "BillingMode": "PAY_PER_REQUEST",
  "AttributeDefinitions": [
    { "AttributeName": "PK", "AttributeType": "S" },
    { "AttributeName": "SK", "AttributeType": "S" },
    { "AttributeName": "GSI1PK", "AttributeType": "S" },
    { "AttributeName": "GSI1SK", "AttributeType": "S" }
  ],
  "KeySchema": [
    { "AttributeName": "PK", "KeyType": "HASH" },
    { "AttributeName": "SK", "KeyType": "RANGE" }
  ],
  "GlobalSecondaryIndexes": [
    {
      "IndexName": "GSI1",
      "KeySchema": [
        { "AttributeName": "GSI1PK", "KeyType": "HASH" },
        { "AttributeName": "GSI1SK", "KeyType": "RANGE" }
      ],
      "Projection": { "ProjectionType": "ALL" }
    }
  ]
}
```

---

**Przykładowy Migrator w Go**
`db/migrator/main.go`
```go
package main

import (
  "context"
  "encoding/json"
  "fmt"
  "os"
  "path/filepath"

  "github.com/aws/aws-sdk-go-v2/config"
  "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func main() {
  ctx := context.Background()
  cfg, err := config.LoadDefaultConfig(ctx)
  if err != nil {
    panic(err)
  }

  client := dynamodb.NewFromConfig(cfg)

  // 1) Wczytaj migrację
  data, err := os.ReadFile("db/migrations/001_create_table.json")
  if err != nil {
    panic(err)
  }

  var input dynamodb.CreateTableInput
  if err := json.Unmarshal(data, &input); err != nil {
    panic(err)
  }

  // 2) Wykonaj migrację (idempotentnie)
  _, err = client.CreateTable(ctx, &input)
  if err != nil {
    fmt.Println("CreateTable error:", err)
  }

  fmt.Println("Migration done:", filepath.Base("db/migrations/001_create_table.json"))
}
```

---

**Uwagi praktyczne**
- Trzymaj kluczowe indexy tylko jeśli są potrzebne w MVP.
- Migracje uruchamiaj przed deployem Lambdy.
- Dane historyczne (mecze) najlepiej indeksować po `CreatedAt/PlayedAt`.
