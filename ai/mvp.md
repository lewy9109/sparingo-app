# MVP – Aplikacja do zarządzania ligą squasha

---

## 1. Użytkownicy i Profile (Fundament)

Nie budujemy Facebooka, ale musimy wiedzieć, kto z kim gra.

### Rejestracja / Logowanie

* Email + hasło
* Logowanie Google / Apple (zalecane dla wygody użytkownika)

### Profil Gracza

* Imię i nazwisko / Nick
* Zdjęcie profilowe (ważne w ligach – kojarzenie twarzy)
* Poziom umiejętności (początkujący / średni / pro – deklaratywnie)
* Dashboard:

    * Nadchodzące mecze
    * Ostatnie wyniki

---

## 2. Zarządzanie Ligami (Serce aplikacji)

To tutaj dzieje się administracja ligi.

### Tworzenie Ligi

* Nazwa
* Opis
* Lokalizacja (klub)

### Role w Lidze

* **Właściciel / Główny Admin**

    * Edycja ligi
    * Usuwanie ligi
    * Nadawanie administratorów

* **Administratorzy**

    * Dodawanie / usuwanie graczy
    * Edycja błędnych wyników

* **Gracze**

    * Grają mecze
    * Zgłaszają wyniki

### Tabela Ligowa

Automatycznie generowany ranking zawierający:

* Punkty
* Liczbę meczów
* Bilans setów
* Bilans małych punktów

---

## 3. Mecze i Wydarzenia (Rozgrywka)

Rozróżniamy dwa typy rozgrywek: ligowe oraz towarzyskie.

### Wydarzenia Ligowe (Kolejki)

* Admin może wygenerować kolejkę (np. każdy z każdym)
* Alternatywnie: system wyzwań (Challenge), gdzie gracze sami dobierają się w pary w ramach ligi

### Mecze Towarzyskie

* Możliwość rozegrania meczu poza ligą
* Nie wpływa na tabelę ligową
* Zapisuje się w historii gracza

### Status Meczu

* Zaplanowany
* W trakcie
* Zakończony

---

## 4. System Wyników i Weryfikacja (Trust but verify)

Kluczowy element zapobiegający konfliktom.

### Wprowadzanie Wyniku

* Obsługa formatu:

    * Best of 3
    * Best of 5
* Wprowadzanie wyników setów (np. 11:9, 11:5, 8:11)

### Flow Potwierdzania

1. Gracz A wpisuje wynik
2. Gracz B otrzymuje powiadomienie „Potwierdź wynik”
3. Jeśli Gracz B potwierdzi → tabela automatycznie się aktualizuje
4. Jeśli Gracz B odrzuci → alert do administratora ligi (konflikt)

---

## 5. Komunikacja (Czat)

W MVP skupiamy się na kontekście, nie budujemy pełnoprawnego komunikatora.

### Czat 1‑na‑1

* Prosty czat tekstowy
* Dostępny z poziomu profilu gracza („Wyślij wiadomość”)

### Kontekst Meczu

* Przycisk „Umów się” przy zaplanowanym meczu
* Otwiera czat z przeciwnikiem

### Powiadomienia Push

* „Masz nową wiadomość”
* „Ktoś dodał wynik meczu z Tobą”

---

# Czego NIE ROBIĆ w MVP (Pułapki)

Te funkcje zwiększają koszt i czas developmentu, a nie są kluczowe na starcie:

* ❌ Rezerwacja kortów
  Integracja z systemami klubów to duża złożoność. W MVP wystarczy pole tekstowe „Gdzie gramy?”.

* ❌ Skomplikowane algorytmy rankingowe (Elo, Glicko-2)
  Na start wystarczy prosty system punktowy:

    * Wygrana = 3 pkt
    * Przegrana = 1 pkt

* ❌ Streaming / Wideo
  Zbyt kosztowne i technologicznie złożone na etap MVP.

---

To jest solidny, realistyczny zakres MVP – wystarczający, by dostarczyć wartość, a jednocześnie nie zabić projektu złożonością.
