# Cleaner (Go) — полная спецификация для исполняющего агента

> **ЭТО ЕДИНСТВЕННЫЙ SOURCE OF TRUTH.**  
> Других файлов проекта у тебя может не быть. Всё нужное — ниже.  
> Язык: **Go 1.22+**. ОС: **Windows**. Module/binary: **`cleaner`** / **`cleaner.exe`**.  
> Все продуктовые решения **заморожены (F\*)**. OPEN-вопросов больше нет. Реализуй как написано.

---

## 1. Цель продукта

CLI на Go:

1. Всегда работает с `C:/Source/combo` (хардкод, независимо от CWD).
2. Обходит только top-папки `combo/<top>/...`; файлы в корне `combo` игнорирует.
3. Скипает top-папки из `C:/Source/combo/cleaned.txt`.
4. Находит и обрабатывает все `.txt`/`.csv` (любой регистр расширения); мусор фильтруется построчно.
5. Обязан получить ответ delete/save (prompt и/или флаги) — иначе крутит вопрос заново.
6. Чистит строки до пары login/password и записывает её как CSV `"login","password"`.
7. Пишет `{stem}_cleaned{ext}` рядом с исходником.
8. Параллелит по файлам (`GOMAXPROCS` воркеров).
9. Live-progress по активным воркерам.
10. После успешной обработки всех запланированных файлов папки — пишет папку в metafile; при ошибках оставляет её для повтора.
11. Итоговая статистика + разбивка причин дропа строк.

---

## 2. Замороженные решения (F*)

| ID | Решение |
|---|---|
| F1 | Выход: CSV `"email","pass"` \| `"username","pass"`; внутренние `"` удваиваются. |
| F2 | Порядок нормализации — §6 (каркас req 1→11 + вставки фильтров). |
| F3 | Пробелы: §6.2 (`unicode.IsSpace` → space → Fields → join `:`). |
| F4 | Файл читается потоково целиком. Если допустимых записей нет либо есть две или более строки из 3+ колонок без распознанной структуры учётных данных (включая неоднозначные `login:pass,часть,...`), число которых не меньше числа допустимых записей, а центральная выборка F16 не проходит — вернуть явную ошибку формата без `_cleaned` и без удаления исходника. Несколько настоящих пар с запятыми в пароле без других признаков формата тоже могут быть отклонены: это безопасный отказ при неоднозначности. |
| F5 | Progress = число **активных** воркеров ≤ GOMAXPROCS. |
| F6 | Metafile: писать top-folder только когда **все запланированные файлы папки успешно обработаны**. При ошибке оставить папку для следующего запуска. |
| F7 | Ответ обязателен: `1`/`delete` или `2`/`save`. Без валидного ответа — переспрашивать вечно. Флаги `-delete`/`-save` задают ответ без prompt, но ответ должен быть (иначе prompt). |
| F8 | `:` внутри обычного пароля не поддерживать; внутри именованных MD5-хэшей с солью из Raccoon Parser поддерживать. |
| F9 | Ban CI: `admin`, `unknown`, `test`, `not_saved`, `not saved`, `notsaved`; для распознанного hash в password проверять только login, чтобы случайная последовательность в hash не удаляла запись. |
| F10 | Drop: `?{3,}`, U+FFFD, `**` (≥2). Ровно `??` — keep. |
| F11 | Фильтры §8 + early junk §9. |
| F12 | Корень `combo` не трогать. |
| F13 | `{stem}_cleaned{ext}`, коллизии `_1`, `_2`, …; если SHA-256 готового результата совпадает с любым существующим `_cleaned_N`, включая номера после пропуска, повторно не создавать файл. Новый результат публиковать без перезаписи существующего; при коллизии сохранить исходник. |
| F14 | Финальный dedupe, ключ `ToLower`, набор отдельный для каждого файла. |
| F15 | Pass = `[!-9;-~]{1,128}` или пароль в именованном формате hash из Raccoon Parser плюс Argon2. Его класс `unknown` не считается подтверждённым hash. Формат проверять целиком, а не только по префиксу. |
| F16 | Все строки оцениваются до EOF. При доминировании многоколоночной таблицы взять центральные 100 строк (если строк меньше — все); если более 70% содержат `http` или `android` без учёта регистра, снять только табличный отказ. Для 500 строк выборка — строки 201–300 (индексы 200–299); 70 из 100 недостаточно. Без допустимой пары файл всё равно отклоняется; обычные пары без маркеров принимаются. |
| F17 | Root хардкод only. Нет `-root`. |
| F18 | Skip файлы с `_cleaned` в имени (перед ext, без учёта регистра). |
| F19 | Ext case-insensitive. |
| F20 | Walk: **follow** directory symlinks/junctions, чтобы находить файлы. Содержимое файлов / URL внутри строк **никогда не «открывать» и не скачивать**. |
| F21 | Глубина рекурсии: без лимита. |
| F22 | Пустые и мусорные строки учитываются причинами drop построчного pipeline. |
| F23 | Лимита размера файла нет. |
| F24 | `cleaned.txt`: UTF-8, одно имя на строку, `\n`. |
| F25 | Metafile append сразу когда папка завершена; atomic write. |
| F26 | Нет `--force` в v1 (редактировать metafile вручную). |
| F27 | Per-file лог `[OK]` / `[ОШИБКА]` — **да**. |
| F28 | Summary: aggregate + **разбивка причин дропа** строк. |
| F29 | Exit: `0` все файлы ok; `1` ≥1 ошибка файла; `2` fatal (нет ROOT и т.п.). Prompt не бывает «fatal без ответа» — только loop. |
| F30 | `len` полей — минимум 5, проверка **после** `"→""`. |
| F31 | TLD-список §8.5 — да, но URL-детектор **не должен** убивать email (§8.4.1). |
| F32 | Encodings: read utf-8-sig→utf-8→cp1251→latin-1; write UTF-8 LF. |
| F33 | Если kept=0: вернуть ошибку формата, **не создавать** cleaned и **не удалять** исходник даже при delete. |
| F34 | Module `cleaner`, binary `cleaner.exe`, Go 1.22+. |
| F35 | Top-folder с **0** найденных `.txt`/`.csv` → **не** писать в metafile. |
| F36 | Двухпольные `LOGIN/PASS` и трёхпольные `URL/LOGIN/PASS` / `LOGIN/PASS/URL` принимаются с разделителями `:`, CSV, `\|`, `;`, tab или whitespace; quoted fields разбираются как CSV. `android:app,LOGIN,PASS`, `android:package:screen:form,LOGIN,PASS` — трёхпольные URL-записи. Строки CSV из 4+ колонок не извлекать как пары, кроме неоднозначного одиночного `LOGIN:PASS` с разделителями внутри пароля; на уровне файла действует F4. Для именованного MD5-хэша с `:` разделитель внутри хэша не разрывает пару. |
| F37 | `go test` обязателен (§12). |
| F38 | Ошибка файла: остальные файлы папки обработать; папку в metafile **не писать**, чтобы повторить её при следующем запуске. Уже совпадающие очищенные результаты не дублировать. |
| F39 | Drop полей: известное файловое расширение, валидный JSON object/array, Base64 длиной ≥48, полноценный email в password, hex hash типовой длины в username, серия ≥6 одинаковых символов. Пароль в распознанном формате hash не удалять по случайным совпадениям этих password-эвристик. Один `@` и серии до 5 символов допустимы. |
| F40 | Перед dedupe/записью применить final guards: расширенный URL/domain regex в password (кроме распознанного hash) и инверсию строгой positive pair validation. Negative lookahead реализовать логически, так как Go RE2 его не поддерживает. |
| F41 | После final regex guards удалить итоговую строку, если она содержит `==@`, `android`, `discord`, `bot` (слова проверять CI) или `//`. Для распознанного hash проверять эти фрагменты только в login; содержимое password-hash не является причиной удаления. |
| F42 | Самый последний шаг после фильтров и dedupe: `^([^:;|]+)[:;|](.+)$` → `"$1","$2"` (Go replacement syntax). |
| F43 | До извлечения пары удалить строку длиннее 2 КиБ. В полях удалить `ENC893*`, `CSCO_WEBVPN_*`, `v1.`, `pbkdf2_`, нераспознанные hash/scrypt-артефакты, UUID, домен в username, файловый путь, query-string и тройной повтор блока. Именованный hash в password не удалять по случайному совпадению с маркером артефакта. |
| F44 | Password длиной ≥30 удалить как opaque artifact, если он не соответствует именованному hash-формату Raccoon Parser или Argon2. Поддержать raw hex 32/40/48/51/56/64/96/128, bcrypt, PHPass, MD5/MD5-APR/MySQL5, SHA-crypt, Drupal, Django SHA, `$HEX[...]`, `$SHA[...]` и MD5 с солью после `:`; у распознанного `MD5:соль` убрать всё после первого `:`, оставив 32 hex MD5. Остальные hash-форматы сохранять целиком. `unknown` из Raccoon Parser, PBKDF2, yescrypt и scrypt crypt не сохранять как известные хэши. Шаблон `32 hex + 8 alnum + _ДД.ММ.ГГГГ` нормализовать до первых 32 hex MD5. Голый hex может также быть ID и по политике сохранения именованных hash-форматов остаётся. |
| F45 | Не-email login длиной ≥40 удалить как metadata. Удалять UUID-фрагменты в полях, `SocialLogin` в password и файловые поля с `.req`, `.csr`, `.xlsm`, `.xlsb`, `.database`, `.sqlite3`, `.mdb`, `.accdb`. |
| F46 | Файл с basename `readme.txt` не планировать на обработку; сравнение без учёта регистра. |
| F47 | Любой login длиной ≥65 удалить как metadata, включая формально валидный email; длина 64 допустима. |
| F48 | Формально валидный email считать URL-мусором, если его local-part до `@` содержит `/` или `\`; обычные email с точками, `+` и допустимыми символами сохранить. |
| F49 | Pipeline идемпотентен: повторная обработка канонической CSV-строки `"login","password"` возвращает ту же строку. |
| F50 | Удалять запись, если password имеет структуру профиля приложения `поставщик_[клиент]_профиль`, `приложение(ProfileN)` или `поставщикSoftware_приложениеStable/ProfileN` (например, `Microsoft_[Edge]_Default`, `SOFTGoogleChrome(Profile1)`, `OperaSoftware_OperaStable`), независимо от поставщика и login. Один лишь суффикс `Application` в login не является основанием для удаления. |
| F51 | Удалять многочастные пути в password длиной от 20 символов, короткие непрозрачные токены вида `число/значение` и метаданные вида `ключ=имя.пакета/путь`. Одинарный `/` или `=` сам по себе допустим. |
| F52 | Удалять помеченные кодированные значения вида `{маркер}_данные` и шаблонные присваивания вида `имя=(значение)` с хвостом из спецсимволов. |
| F53 | Любое значение password, начинающееся с `[`, удалять как служебный токен/плейсхолдер независимо от содержимого и закрывающей `]`. |

---

## 3. Платформа

| Тема | Правило |
|---|---|
| Go | 1.22+, module `cleaner` |
| Regex | `regexp` RE2 only |
| Запрещено RE2 | lookahead/lookbehind/backref |
| regexp2 | не использовать |
| Workers | `runtime.GOMAXPROCS(0)` |
| ROOT | `C:/Source/combo` |

CLI flags:

```
-delete    // эквивалент ответа delete; prompt не нужен
-save      // эквивалент ответа save; prompt не нужен
```

Если оба сразу — fatal exit 2.  
Если ни одного — интерактивный prompt до валидного ответа.  
Валидные ответы prompt: `1`, `delete`, `2`, `save` (trim, CI).

---

## 4. Константы

```
ROOT            = C:/Source/combo
METAFILE        = C:/Source/combo/cleaned.txt
MIN_FIELD_LEN   = 4
MAX_LOGIN_LEN   = 254
MAX_EMAIL_LOCAL = 64
MAX_PASS_LEN    = 128
BASE64_FULL_MIN = 40
HEX_FULL_MIN    = 80
PROGRESS_HZ     = 1s
```

---

## 5. E2E оркестрация

### 5.1 Bootstrap

1. Если ROOT нет / не dir → fatal exit 2.  
2. Прочитать metafile → set top-folder names (нет файла = пусто).  
3. Резолв delete/save: flags XOR prompt-loop (F7).

Prompt текст:

```
Удалять исходные файлы после обработки? (1/delete = удалить, 2/save = оставить):
```

### 5.2 Discovery

1. Immediate child directories of ROOT = top-folders.  
2. Файлы в ROOT — ignore.  
3. Top в metafile — skip.  
4. Walk внутри top: follow symlinks to dirs/files (F20); без лимита глубины.  
5. Кандидаты: ext `.txt`/`.csv` CI; **не** содержат `_cleaned` в имени файла (F18) и не равны `readme.txt` CI (F46).  

### 5.3 Планирование файлов

Все кандидаты из §5.2 планируются в worker pool после удаления повторных путей к одному файлу через ссылки. Запуск удерживает блокировку `.cleaner.lock` в ROOT; второй экземпляр завершится с ошибкой до чтения metafile. Файл целиком проходит pipeline; результат сначала пишется во временный файл с уникальным именем. При отсутствии допустимых записей возвращается ошибка формата. Две и более строки из 3+ колонок без распознанной структуры учётных данных (включая неоднозначные пары с запятыми в пароле), число которых не меньше числа допустимых записей, обычно тоже приводят к отказу; исключение — центральная выборка F16. При ошибке временный файл удаляется, cleaned не создаётся, оригинал сохраняется. Папка остаётся вне metafile для повторной попытки.

### 5.4 Планирование и metafile

- Группировка найденных файлов по top.  
- Если у top `planned == 0` → **не** писать в metafile (F35), идти дальше.  
- Worker pool на все jobs.  
- На top: `done++` на каждый finished job (ok или err).  
- Когда `done == planned` и ошибок в папке нет → **appendMeta(top)** (F6/F38).
- Ошибки копятся глобально для exit code.

### 5.5 Один файл

1. Decode (F32) и потоково применить pipeline ко всем строкам.
2. Писать принятые записи во временный файл и считать признаки многоколоночной таблицы.
3. Если допустимых записей нет или повторяющаяся таблица преобладает и не проходит F16 → ошибка формата, временный файл удалить, исходник сохранить.
4. Иначе использовать идентичный существующий результат или записать новый по F13; counters: kept, invalid, duplicates, reason histogram.
5. Если `deleteSource` → удалить source только после записи результата.
6. Per-file лог (F27).

OK строка:

```
[OK] name.txt -> оставлено: K, удалено: R, дубли: D; исходник удалён|сохранён
[ОШИБКА ФОРМАТА] name.txt: неподходящий формат файла: ...
```

`R = invalid + duplicates`.

### 5.6 Progress

Блок из W строк (busy workers), раз в 1s:

```
обработка <filename> в папке <top> - <sec> секунд
```

Перерисовка in-place (ANSI/`\r`). Не раздувать числом файлов в очереди.

### 5.7 Summary

```
Готово.
Успешно обработано файлов: <ok>
Ошибок: <fail>
Всего оставлено строк: ...
Всего удалено строк: ...
Всего удалено дублей: ...

Причины удаления строк:
  <reason>: <count>
  ...

Исходные TXT/CSV удалялись только после создания непустого результата.
```
или сообщение что режим save.

Резоны (минимум; имена стабильные):

```
empty
early_url_full
early_jwt
early_android
early_base64
early_hex
early_shortener
oversize_line
no_separator
broken_utf8_or_qqq
stars
url_after_colon
invalid_pair
duplicate
login_eq_pass
short_field
banned_substring
filename_field
json_field
long_base64_field
email_in_password
hash_in_username
repeated_character_run
artifact_field
final_url_match
final_invalid_format
final_forbidden_fragment
```

Строка может иметь одну primary reason (первая сработавшая стадия).

Exit: F29.

---

## 6. Пайплайн строки

### 6.0 Порядок

| # | Стадия | Действие |
|---|---|---|
| P0 | trim/empty | TrimSpace; empty → drop `empty` |
| P1 | early junk | §9.1–9.3, 9.5–9.7 |
| P2 | extract | распознать трёхпольные форматы §6.1, затем email:pass extract §6.3 |
| P3 | req2 | `" :"` / `": "` → `:` |
| P4 | req3 | whitespace normalize §6.2 |
| P5 | req4 | снова extract §6.3 |
| P6 | req5 | last two `:` segments §6.4 |
| P8 | ??? / U+FFFD / `**` | drop |
| P8.1 | MD5+suffix repair | F44 → оставить первые 32 hex |
| P9 | URL after first `:` | §8.1 (с защитой email) |
| P10 | validate pair | §8.2 |
| P13 | `"→""` | |
| P14 | login==pass | EqualFold → `login_eq_pass` |
| P15 | len&lt;5 | после P13 → `short_field` |
| P15.1 | junk fields | F39/F43/F44/F45/F47 → соответствующий reason |
| P16 | ban | §6.5 → `banned_substring` |
| P16.1 | final guards | F40 → `final_url_match` / `final_invalid_format` |
| P16.2 | forbidden fragments | F41 → `final_forbidden_fragment` |
| P17 | dedupe | final line ToLower, per file |
| P18 | CSV output | F42, только после всех проверок |

Набор дедупликации — один на файл; в него попадают только строки, прошедшие все фильтры.

### 6.1 Извлечение пары из разделённых полей

Два поля трактуются как `LOGIN/PASS`. Для трёх и более полей поддерживаются оба порядка: `URL/LOGIN/PASS` и `LOGIN/PASS/URL`. Кроме `:`, допустимы CSV (включая quotes и doubled quotes), `|`, `;`, tab и whitespace. Ориентация многопольной записи определяется только когда URL-поле распознано по §8.4, `android:` или схеме `://`; выход всегда `LOGIN:PASS`.

### 6.2 normalizeSeparators (P3+P4)

```
1. Каждый unicode.IsSpace → ' '
2. Пока меняется: replace " :" и ": " → ":"
3. Если есть ' ': fields = strings.Fields(line); line = strings.Join(fields, ":")
```

P3 = шаг 2; P4 = шаг 3. P2 (extract) **до** P4.

Примеры:

| Вход | После P3–P4 | После P6 |
|---|---|---|
| `user@mail.com:pass1` | same | same |
| `prefix user@mail.com:pass1` | `prefix:user@mail.com:pass1` | `user@mail.com:pass1` |
| `user pass` | `user:pass` | `user:pass` |
| `http://x.com user@a.b p` | `http://x.com:user@a.b:p` | `user@a.b:p` |
| `user pass1234 https://x.com` | распознаётся до P4 | `user:pass1234` |

### 6.3 Extract email:pass (P2/P5)

```
^((?:[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}):[^:]*):.*$
```

Submatch group 1 → новая строка. Иначе без изменений.  
В Go: `FindStringSubmatch`, не backref replace.

### 6.4 Last two (P6)

```
parts = Split(line, ":")
nonEmpty = filter len>0
if len(nonEmpty) < 2: drop no_separator (или на P7)
line = nonEmpty[len-2] + ":" + nonEmpty[len-1]
```

### 6.5 Ban (P16)

```go
s := strings.ToLower(line)
for _, b := range []string{"admin","unknown","test","not_saved","not saved","notsaved"} {
    if strings.Contains(s, b) { drop }
}
```

### 6.6 Split login/pass

```go
i := strings.IndexByte(line, ':')
login, pass := line[:i], line[i+1:]
```

Если `pass` содержит `:` → drop `invalid_pair` (F8).

---

## 7. Валидация пары (P10)

POSITIVE keep если:

1. `1 <= len(login) <= 254`  
2. Login = **email** ИЛИ **username**:

**Email** (если содержит `@`):

```
(?i)^[A-Z0-9!#$%&'*+/=?^_`{|}~-]+(?:\.[A-Z0-9!#$%&'*+/=?^_`{|}~-]+)*@(?:[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?\.)+[A-Z]{2,63}$
```

+ local part (до `@`) ≤ 64.

**Username** (если нет `@`):

```
(?i)^[A-Z0-9](?:[A-Z0-9._-]{0,62}[A-Z0-9])?$
```

3. Pass — один из вариантов:

```
^[!-9;-~]{1,128}$
(?i)^\$(?:2[ABY]|ARGON2(?:ID|I|D)|SCRYPT|[1567Y])\$[!-9;-~]{1,240}$
```

Иначе drop `invalid_pair`.

---

## 8. URL-детектор (критично про email)

### 8.1 P9 — URL после первого `:`

Проверяй **только password** (после первого `:`).  
Если `containsURL(pass)` → drop `url_after_colon`.

### 8.4 `containsURL(s)` 

Ищи (CI), порядок альтернатив:

1. `https?://`  
2. `www.[a-z0-9-]+`  
3. domain + TLD из §8.5: `[a-z0-9-]+\.(?:TLD)(?:/|\b)`  

### 8.4.1 Защита email (F31) — ОБЯЗАТЕЛЬНО

Доменный паттерн (альтернатива 3) **не должен** срабатывать на email.

Канон реализации (RE2-safe):

- Найти все матчи паттерна 3.  
- Для каждого матча посмотреть байт/руну **сразу перед** началом матча.  
- Если это `@` → матч **игнорировать** (это `mail.com` в `user@mail.com`).  
- Также игнорировать матч, если весь `s` или проверяемый фрагмент сам является валидным email (§7 email regex) — belt & suspenders.

Примеры:

| Фрагмент | containsURL? |
|---|---|
| `user@mail.com` | **нет** |
| `http://mail.com` | да |
| `www.mail.com` | да |
| `goto.mail.com/x` | да |
| `notmail.com` | да (domain) |
| `user@mail.com:pass1` как login+pass | login email валиден; pass без URL → keep |

Early junk 9.1 full-line URL: строка целиком URL → drop. Строки `user@mail.com:pass1` и `https://site/path:user1:pass1` full-line URL? **нет**.

### 8.5 TLD list

```
com|org|net|info|online|ai|dev|gg|icu|xyz|io|co|me|app|site|club|top|pro|biz|tv|cc|
ru|ua|by|kz|su|edu|gov|mil|int|arpa|us|uk|de|fr|es|it|nl|pl|cz|sk|ch|at|be|se|no|fi|
dk|ie|pt|gr|tr|il|jp|kr|cn|in|br|au|nz|ca|mx|ar|cl|za|sg|hk|tw|th|vn|id|my|ph|
shop|store|blog|cloud|tech|space|fun|live|news|media|email|zip|mov|click|link|
page|pw|to|ws|fm|ly|gd|gl|tk|ml|ga|cf|gq|work|company|earth|world|today|agency
```

---

## 9. Early junk (P1) + P8

Для распознанной строки `URL:LOGIN:PASS` или `LOGIN:PASS:URL` проверки 9.2–9.7 применяются только к `LOGIN:PASS`; URL-поле не должно загрязнять пару.

### 9.1 Full-line URL

Если после Trim вся строка = URL по §8.4 (с email-защитой) → `early_url_full`.  
Не использовать naive domain find без проверки `@`.

### 9.2 JWT

```
\beyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]*\b
```

→ `early_jwt`

### 9.3 Android

```
(?i)(android[_\- ]?id|androidid)\s*[=:]\s*[A-Za-z0-9+/=_\-]{16,}
(?i)android[_\- ]?id[=:][a-f0-9]{16,}
```

→ `early_android`

### 9.4 / P8 Broken + stars

- `'\uFFFD'` или `???` (Contains `???`) → `broken_utf8_or_qqq`  
- Contains `**` → `stars`

### 9.5 Base64 full line

```
^[A-Za-z0-9+/=_\-]{40,}$
```

→ `early_base64`

### 9.6 Hex full line

```
^[A-Fa-f0-9]{80,}$
```

→ `early_hex`

### 9.7 Shortener/UTM

```
(?i)(bit\.ly|tinyurl|goo\.gl|t\.co|is\.gd|buff\.ly|ow\.ly|shorturl|adf\.ly|utm_source=|utm_medium=|utm_campaign=|fbclid=|gclid=)
```

→ `early_shortener`

### 9.8 No separator (P7)

После извлечения §6.1 нет `:` → `no_separator`. Двухпольные `;`/`|` не спасают (F36).

---

## 10. IO

### 10.1 Read

По порядку: UTF-8 BOM strip → UTF-8 → cp1251 (`charmap.Windows1251`) → latin-1.  
Fail → ошибка файла.

### 10.2 Write

UTF-8, LF, без BOM. Temp+rename предпочтительно.

### 10.3 Output name

`stem_cleaned` + ext; если любой существующий результат, включая номер после пропуска, идентичен, не создавать дубль; иначе выбрать первый свободный `_1`, `_2`, …

---

## 11. Metafile

- Path: `C:/Source/combo/cleaned.txt`  
- UTF-8, одно имя top на строку  
- Писать когда `done==planned` и все файлы успешны для top с `planned>0`
- Не писать если `planned==0`  
- Не писать при ошибках файлов; следующий запуск повторит папку
- Переписывать через временный файл и переименование, не удаляя старый `cleaned.txt` перед попыткой
- Нет `--force`

---

## 12. Обязательные тесты

| Вход | Ожидание |
|---|---|
| `user@mail.com:pass123` | keep same |
| `prefix:user@mail.com:pass123` | `user@mail.com:pass123` |
| `prefix user@mail.com:pass123` | `user@mail.com:pass123` |
| `user pass1234` | `user:pass1234` |
| `user:pa"ss12` | `user:pa""ss12` |
| `user:http://site.com` | drop |
| `user:www.site.com` | drop |
| `user:user` | drop |
| `ab:cdefgh` | drop |
| `abcd:ef` | drop |
| `test@x.com:abcdef` | drop ban |
| `good@x.com:not_saved` | drop |
| `good@x.com:notSaved1` | drop |
| `good@x.com:aa??bb` | keep |
| `good@x.com:aa???bb` | drop |
| `good@x.com:aa**bb` | drop |
| `user@mail.com:secret99` | keep (**email не URL**) |
| `plainword` | drop |
| `https://example.com/x` | drop early |
| `https://example.com/x:user123:pass1234` | `user123:pass1234` |
| `https://example.com/?utm_source=x:user123:pass1234` | `user123:pass1234` |
| `user123:pass1234:https://example.com/x` | `user123:pass1234` |
| `https://example.com/x\|user123\|pass1234` | `user123:pass1234` |
| `user123;pass1234;https://example.com/x` | `user123:pass1234` |
| `user;pass` | drop |
| `ADMIN@x.com:abcdef` | drop |

`containsURL("user@mail.com") == false`  
`containsURL("http://mail.com") == true`

Файл из мусорных строк всё равно планируется; pipeline не создаёт результат, выводит ошибку формата и сохраняет оригинал по F33.

---

## 13. Структура проекта

```
go.mod
cmd/cleaner/main.go
internal/pipeline/   // + tests
internal/discover/
internal/metafile/
internal/worker/
internal/encode/
README.md
AGENT_PROJECT_SPEC.md
```

Упрощение пакетов ок, если тесты pipeline есть.

---

## 14. Фазы работы агента

1. Skeleton + flags + prompt loop  
2. Discover + планирование всех кандидатов + tests  
3. Pipeline + URL/email tests  
4. File IO + unsupported-format policy
5. Workers + progress  
6. Metafile + summary reasons + exit codes  
7. `go test ./...`, README, ручной смоук

---

## 15. MUST / MUST NOT

**MUST:** F1–F49; Go RE2; final case-insensitive dedupe; final URL/pair/fragment guards; final CSV transform; email-safe URL detect; prompt/flags ответ обязателен; empty kept → ошибка формата и сохранение source; metafile only after all files succeed; reason breakdown; tests.

**MUST NOT:** regexp2; lookahead; дроп за `??`; писать metafile для папок с 0 planned; удалять source при kept=0; трогать файлы в корне combo; пропускать целый файл по выборке содержимого; принимать domain-match сразу после `@`.

---

## 16. Оптимизации (опционально)

Atomic metafile. Файлы читаются и записываются потоково; корректность важнее.

---

## 17. Pseudocode

```
mode = flags or promptLoop() // delete|save
doneSet = readMeta()
jobs = []
plan = map[top]int

for top in childDirs(ROOT):
  if top in doneSet: continue
  files = walkFollow(top) // txt/csv, skip *_cleaned*
  if len(files)==0: continue // NO meta write
  plan[top] = len(files)
  jobs += files

pool(GOMAXPROCS, jobs, func(j):
  progressAdd(j)
  st, err = process(j, mode)
  progressDel(j)
  if err != nil { fail++; failed[top]=true; logErr } else { ok++; logOK; acc(st) }
  finished[top]++
  if finished[top]==plan[top] && !failed[top] { appendMeta(top) }
)

summary(...); exit(fail>0 ? 1 : 0)
```

`process`: записать cleaned только при подходящем формате; удалить source только после записи результата.

---

## 18. Сдача

1. Компилируемый проект, `cleaner.exe`.  
2. `go test ./...` green.  
3. README: build/run, flags `-delete`/`-save`, prompt, ROOT path.  
4. Короткий отчёт: что сделано, как прогнаны тесты.

---

Конец спецификации. OPEN-вопросов нет.
