# WeightedFormulaDecisionEngine — алгоритм принятия решения

`Engine.Decide` (`engine.go`) детерминированно превращает `news.NewsAnalysis`
(результат LLM-анализа новости) в `trading.TradingSignal` (рекомендация
`Buy`/`Sell`/`Watch`/`Hold` по конкретной бумаге). Никакого ML/LLM внутри —
только конфигурируемая формула, что делает решение воспроизводимым и
объяснимым (см. поле `Rationale` в результате).

## Вход и выход

Вход — `news.NewsAnalysis`: `FundamentalSentiment`, `Surprise`, `ImpactScore`,
`Confidence` (оба в `[0,1]`), `TimeHorizon`, `EventType`, `PositiveFactors`/
`NegativeFactors`, `Ticker *trading.Ticker` (может быть `nil`, если модель не
привязала новость к конкретному инструменту).

Выход — `trading.TradingSignal` с полями `Action`, `Track`, `FinalScore`,
`Strength`, `Rationale`, детерминированным `ID` и `ExpiresAt`.

Конфигурация (`config.OrchestratorConfig`) задаёт все веса/пороги — в коде
нет ни одного захардкоженного числа.

## Шаг 1. `sentimentScore` — направление по тональности

`FundamentalSentiment` проецируется на отрезок `[-1, 1]`:

| Sentiment  | Score                                                                 |
|------------|------------------------------------------------------------------------|
| `positive` | `+1`                                                                    |
| `negative` | `-1`                                                                    |
| `neutral`  | `0`                                                                     |
| `mixed`    | `clamp((#PositiveFactors − #NegativeFactors) / (#Positive + #Negative), -1, 1) × MixedSentimentPenalty` |
| неизвестно | `0`                                                                     |

Для `mixed` берётся разность количества факторов "за" и "против", нормированная
их суммой (доля перевеса), и приглушается `MixedSentimentPenalty`
(по умолчанию `0.5`) — смешанная новость не должна давать такой же сильный
сигнал, как однозначно `positive`/`negative`. Если факторов нет вообще
(`#Positive + #Negative == 0`), результат — нейтральный `0` (без деления на
ноль).

## Шаг 2. `surpriseScore` и вес surprise

`Surprise` (сравнение факта с ожиданиями рынка) проецируется так же:
`positive → +1`, `negative → -1`, `neutral`/`unknown` → `0`.

Отдельно вычисляется **эффективный вес** surprise-компонента:

```
surpriseWeightEff = Weights.Surprise × (Surprise == unknown ? UnknownSurprisePenalty : 1)
```

По умолчанию `UnknownSurprisePenalty = 0` — если модель явно не смогла
сравнить факт с рыночными ожиданиями, surprise-компонент **отключается**, а
не подменяется нейтральным значением. Это отличается от `surprise = neutral`
(там ожидания сравнивались и совпали) — оба в итоге дают вклад `0` в
`directionalRaw`, но по разным причинам, и при других `UnknownSurprisePenalty`
(например, частичном приглушении, а не полном отключении) поведут себя
по-разному.

## Шаг 3. `directionalRaw` — направленная сила сигнала

```
directionalRaw = Weights.Sentiment × sentimentScore + surpriseWeightEff × surpriseScore
```

`Weights.Sentiment + Weights.Surprise == 1` (инвариант конфигурации,
проверяется `config.Validate()`), поэтому `directionalRaw ∈ [-1, 1]`.

## Шаг 4. `FinalScore` — импакт, уверенность и тип события как множители

```
FinalScore = directionalRaw × ImpactScore × Confidence × EventModifiers[EventType]
```

`ImpactScore` и `Confidence` (оба в `[0,1]`) действуют как **понижающие**
множители: важная, но неуверенно оценённая новость, или уверенная, но
незначимая — никогда не дадут сильный сигнал, даже при однозначной
тональности. `EventModifiers` — поправочный коэффициент по категории события
(`earnings`, `dividend`, `m&a`, ...) из конфига; если `EventType` не найден в
карте (например, ещё не описан в `configs/config.yaml`), модификатор по
умолчанию — `1` (без искажения).

## Шаг 5. Выбор `Track` по `TimeHorizon`

| TimeHorizon        | Track       |
|---------------------|-------------|
| `intraday`          | `intraday`  |
| `1-3d`, `1-2w`       | `swing`     |
| `1-3m`              | `position`  |
| не распознан/пусто  | `unknown` (нет `ThresholdSet` → см. гейты) |

У каждого `Track` — свой набор порогов (`ThresholdSet`) в
`configs/config.yaml`: `MinConfidence`, `MinImpact`, `BuyThreshold`,
`SellThreshold`, `WatchThreshold`, `SignalTTL`, `Cooldown`.

## Шаг 6. Гейты — жёсткий `Hold` до сравнения с порогами

Если хотя бы одно условие верно, действие — `Hold`, **независимо от
`FinalScore`**:

- для `TimeHorizon` нет соответствующего `Track`/`ThresholdSet` (горизонт не
  распознан);
- `Confidence < MinConfidence` (модель сама неуверена в оценке);
- `ImpactScore < MinImpact` (событие признано незначимым).

Гейты стоят раньше сравнения с торговыми порогами намеренно: сильный
`FinalScore`, посчитанный на основе низкокачественного анализа, не должен
приводить к сделке.

## Шаг 7. Пороги `Buy` / `Sell` / `Watch`

Если гейты пройдены, `Action` определяется по `FinalScore` для порогов
выбранного `Track`:

```
FinalScore ≥ BuyThreshold        → Buy
FinalScore ≤ −SellThreshold      → Sell
|FinalScore| ≥ WatchThreshold    → Watch
иначе                            → Hold
```

(`BuyThreshold > WatchThreshold > 0` — инвариант, проверяемый
`config.Validate()`.)

## Шаг 8. `Strength`

```
Strength = min(1, |FinalScore|)
```

Используется вниз по пайплайну (например, `PositionSizer`) как показатель
"насколько сильно" рекомендовано действие, отдельно от его направления.

## Шаг 9. `Ticker == nil` — принудительный `Watch`

Если `NewsAnalysis.Ticker == nil` (модель не смогла привязать новость к
конкретному инструменту), `Action` **безусловно переопределяется на
`Watch`** — даже если пороги выше насчитали `Buy`/`Sell`. Оркестратор не
отправляет брокеру сигналы без инструмента; `Watch` фиксирует, что новость
значима, но исполнять её нечем.

## `SignalID` — идемпотентность

```
SignalID = sha256(NewsID + EngineName + OrchestratorConfig.Version)
```

Одна и та же новость с тем же движком и той же версией конфигурации всегда
даёт один и тот же `SignalID` — это делает сохранение сигнала в
`SignalRepository` идемпотентным (повторная обработка той же новости не
создаёт второй сигнал). Смена `Version` в конфиге (например, при изменении
весов/порогов формулы) намеренно меняет `SignalID` — старые и новые сигналы
по одной и той же новости не считаются одним и тем же решением.

## Пример расчёта

Новость: `sentiment=positive`, `surprise=positive`, `impact=0.9`,
`confidence=0.9`, `event=earnings` (модификатор `1.0`), `horizon=intraday`,
тикер задан. Веса: `Sentiment=0.6`, `Surprise=0.4`. Пороги `intraday`:
`MinConfidence=0.6`, `MinImpact=0.5`, `BuyThreshold=0.5`.

```
sentimentScore = +1
surpriseScore  = +1, surpriseWeightEff = 0.4 (surprise не unknown)
directionalRaw = 0.6×1 + 0.4×1 = 1.0
FinalScore     = 1.0 × 0.9 × 0.9 × 1.0 = 0.81
```

Гейты пройдены (`0.9 ≥ 0.6`, `0.9 ≥ 0.5`). `0.81 ≥ BuyThreshold(0.5)` →
**Action = Buy**, `Track = intraday`, `Strength = min(1, 0.81) = 0.81`.
