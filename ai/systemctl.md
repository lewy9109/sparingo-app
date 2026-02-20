# Uruchomienie aplikacji Go jako usługi systemd

Poniższa instrukcja pokazuje jak uruchomić aplikację Go jako usługę systemową przy użyciu `systemctl`.

---

## 1️⃣ Zbuduj binarkę

Na VPS w katalogu projektu:

```bash
go build -o sqoush-app
```

Upewnij się, że plik ma prawa wykonywania:

```bash
chmod +x sqoush-app
```

Przetestuj lokalnie:

```bash
PORT=20266 ./sqoush-app
```

---

## 2️⃣ Utwórz plik usługi systemd

```bash
sudo nano /etc/systemd/system/sqoush.service
```

Wklej konfigurację:

```ini
[Unit]
Description=Sqoush Go App
After=network.target

[Service]
User=krystian
WorkingDirectory=/home/krystian/sqoush-app
ExecStart=/home/krystian/sqoush-app/sqoush-app
Environment=PORT=20266
# Jeśli używasz Postgresa, odkomentuj i ustaw DSN
# Environment=POSTGRES_DSN=postgres://user:pass@localhost:5432/dbname?sslmode=disable
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

⚠️ Upewnij się, że:

* `User` to poprawny użytkownik systemowy
* `WorkingDirectory` to właściwa ścieżka
* `ExecStart` zawiera pełną (absolutną) ścieżkę

---

## 3️⃣ Przeładuj konfigurację systemd

```bash
sudo systemctl daemon-reload
```

---

## 4️⃣ Włącz autostart usługi

```bash
sudo systemctl enable sqoush
```

---

## 5️⃣ Uruchom usługę

```bash
sudo systemctl start sqoush
```

---

## 6️⃣ Sprawdź status

```bash
sudo systemctl status sqoush
```

---

## 7️⃣ Podgląd logów

Najważniejsze przy debugowaniu:

```bash
journalctl -u sqoush -f
```

---

## 🔐 Dodanie zmiennych środowiskowych (Postgres)

Masz dwie poprawne metody.

---

### Opcja 1 – Bezpośrednio w pliku usługi

Edytuj plik:

```bash
sudo nano /etc/systemd/system/sqoush.service
```

W sekcji `[Service]` dodaj:

```ini
Environment=POSTGRES_DSN=postgres://user:password@localhost:5432/sqoush?sslmode=disable
Environment=POSTGRES_MIGRATIONS_DIR=/home/krystian/sqoush-app/migrations/postgres
```

⚠️ Ważne:

* W systemd **nie używaj ścieżek względnych** (`./migrations/...`).
* Zawsze podawaj pełną ścieżkę absolutną.

Po zmianach wykonaj:

```bash
sudo systemctl daemon-reload
sudo systemctl restart sqoush
```

---

### Opcja 2 – Plik EnvironmentFile (czystsze rozwiązanie)

Utwórz plik:

```bash
nano /home/krystian/sqoush-app/.env
```

Wklej:

```bash
POSTGRES_DSN=postgres://user:password@localhost:5432/sqoush?sslmode=disable
POSTGRES_MIGRATIONS_DIR=/home/krystian/sqoush-app/migrations/postgres
PORT=20266
```

Następnie w pliku `sqoush.service` dodaj w sekcji `[Service]`:

```ini
EnvironmentFile=/home/krystian/sqoush-app/.env
```

I zrestartuj usługę:

```bash
sudo systemctl daemon-reload
sudo systemctl restart sqoush
```

---

## ✅ Gotowe

Po wykonaniu tych kroków aplikacja:

* będzie miała dostęp do bazy Postgres
* poprawnie znajdzie katalog migracji
* uruchomi się automatycznie po restarcie VPS
* będzie restartowana przy błędzie
* będzie działać na porcie 20266



---



🔧 Krok 4 — Aktywacja
sudo ln -s /etc/nginx/sites-available/twojadomena /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
🔐 Krok 5 — Certyfikat Let's Encrypt
sudo certbot --nginx -d twojadomena.pl

Certbot sam wstawi ścieżki do certyfikatów.

🔥 Jeśli KONIECZNIE chcesz publiczny port 20266

To musisz zrobić tak:

Nginx → listen 20266 ssl;

Go → zmienić na inny port np 127.0.0.1:3000

Konfiguracja wtedy:

server {
    listen 20266 ssl;
    server_name twojadomena.pl;

    ssl_certificate /etc/letsencrypt/live/twojadomena.pl/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/twojadomena.pl/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:3000;
    }
}

Ale powiem wprost:
Używanie 20266 jako publicznego portu HTTPS jest dziwne i niepotrzebne.
Standard to 443.§
