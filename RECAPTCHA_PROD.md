**Instrukcja: reCAPTCHA v3 na produkcji**

1. **Utwórz klucze produkcyjne**
   - W konsoli Google reCAPTCHA utwórz nowy klucz v3.
   - Dodaj swoją docelową domenę (np. `twojadomena.pl`, `www.twojadomena.pl`).

2. **Ustaw zmienne środowiskowe na serwerze**
   ```bash
   RECAPTCHA_ENABLED=true
   RECAPTCHA_SITE_KEY=twoj_site_key
   RECAPTCHA_SECRET_KEY=twoj_secret_key
   RECAPTCHA_MIN_SCORE=0.5
   ```
   - Domyślnie reCAPTCHA jest wyłączona (`RECAPTCHA_ENABLED=false` lub brak tej zmiennej).
   - `RECAPTCHA_MIN_SCORE` możesz dostroić (np. 0.3–0.7) po obserwacji ruchu.

3. **Włącz HTTPS**
   - reCAPTCHA v3 wymaga HTTPS w produkcji.

4. **Upewnij się, że akcja się zgadza**
   - Frontend wysyła `action: "register"`.
   - Backend weryfikuje, że `action` z odpowiedzi reCAPTCHA to `register`.

5. **Zweryfikuj działanie**
   - Zarejestruj konto na produkcji.
   - Jeśli rejestracja odrzucana, sprawdź:
     - czy domena jest dodana w reCAPTCHA,
     - czy `site key` i `secret key` są poprawne,
     - czy `RECAPTCHA_MIN_SCORE` nie jest zbyt wysoki.

6. **(Opcjonalnie) Ogranicz host**
   - Możesz dodatkowo porównywać `hostname` zwrócony przez reCAPTCHA z Twoją domeną.
