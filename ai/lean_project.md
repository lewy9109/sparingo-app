# Problem	
Trudności w prowadzeniu ligi squashowej, brak historii wyników, "papierowe" tabele, brak weryfikacji wyników.

# Rozwiązanie	
Sqoush App - Lekka aplikacja PWA do zarządzania ligą i wyzwań.
# Użytkownicy	
20 graczy squasha (znajomi), 1-2 administratorów ligi.
# Kluczowe Funkcje (MVP)

1. Logowanie (Google/Email).

2. Lista graczy i Tabela Ligi.

3. Dodawanie wyniku (Sety).

4. Potwierdzanie wyniku przez przeciwnika.

5. Historia meczów.
   Architektura

# Backend:
Go (AWS Lambda).

# Frontend: 
Templ + HTMX (Server Side Rendering).

# DB: 
DynamoDB (On-Demand).

# Styl: 
Tailwind + DaisyUI (Mobile First).
# Koszty
0 PLN (AWS Free Tier: 1mln requestów Lambda, 25GB DynamoDB).
Next Step	Wdrożenie MVP, zebranie feedbacku od 20 graczy, ewentualna monetyzacja później.

# Struktura projektu
```
├── main.go              # Punkt wejścia (serwer HTTP)
├── components/          # Pliki .templ (widoki i komponenty)
│   ├── layout.templ     # Główny szablon (HTML head, nawigacja)
│   ├── league_card.templ# Komponent karty ligi
│   └── chat.templ       # Fragmenty czatu
├── static/              # Wygenerowany CSS i statyczne JS (htmx.min.js)
├── db/                  # Logika bazy danych (SQLite/DynamoDB)
├── tailwind.config.js   # Konfiguracja stylów
└── Makefile             # Twoje centrum dowodzenia (build, run)
```
