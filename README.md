# SLO exporter


## Navrh klasifikace

```
                                   static config
                                     +-------+
                                     |       |
                                     |       |
                                     |       +----------+
                                     |       |          |
                                     +-------+          |
                                                      +-v------+
                                                      | cache  |            +--------------+
                                                      |        |            |              |
+--------------+                                      +-^----+-+            | 70% critical |
| nginx log    |                                        |    |              | 50% warning  |
| processor    +-------+ event                          |    |              |              |
|              |       |                                |    |              +------+-------+
+--------------+       |                                |    |                     |
                       |                                |    |                     |         (classified)                 SLO
+--------------+       |   +--------+      event    +---+----v---+   event   +-----v-------+    event     +-----------+   event    +----------------+
| envoy log    |       |   | event  |               | dynamic    |           | statistical |              | event     |            | Prometheus     |
| receiver     +-----------> filter +---------------> classifier +-----------> classifier  +--------------+ validator +------------> SLO exporter   |
|              |       |   |        |               |            |           |             |              |           |            |                |
+--------------+       |   +--------+               +------------+           +-------------+              +-----------+            +----------------+
                       |
+--------------+       |
| prometheus   |       |
| query        +-------+ event
| processor    |
+--------------+
```

flow:
1. Pri startu se nahraje do cache pocatecni stav ze staticke konfigurace
1. Do dynamickyho classifieru prichazi event:
   1. je klasifikovany? (ma vyplnene slo_ atributy)
      - ano: 
         1. zapise se do cache spolu s jeho normalized identifikatorem
      - ne: 
         1. dotaze se cache na exact match, matchuje?
            - ano: 
               1. prida eventu slo_ data a posle ho dal
            - ne: 
               1. zkusi match regularama, matchuje?
                  - ne: 
                     1. posle event dal beze zmeny
                  - ano: 
                     1. zapise nalezena slo_ data spolu se svym normalizovanym id do exact matchu
                     1. prida eventu slo_ data a posle ho dal
1. Do statistickeho classifieru prijde event
   1. je klasifikovany?
      - ano: 
         1. inkrementuje pro dane slo_ data statistiky
         1. posle ho dal
      - ne: 
        1. na zaklade statistik priradi slo_ data eventu
        1. inkrementuje metriky ze neco klasifikoval pro dany slo_ data
        1. posle ho dal
            
## Format staticke konfigurace
Zapis formou jsonu ale muze byt napr v redisu nebo pro MVP v pameti.
Kdyz senastartuje tak se do tohodle naloadujou ty SLo klasifikace stavajici.
```json
{
"exact_match": {
   "POST:/api/v1/graphQL?operationName=adsList": {"slo_domain": "test-domain", "slo_app": "test_app", "slo_class": "critical"}
   "POST:/api/v1/graphQL?operationName=adsList&severity=critical": {"slo_domain": "test-domain", "slo_app": "test_app", "slo_class": "low"}
},
"regular_expression_match": [
   ["POST:/api/v1/graphQL?.*", {"slo_domain": "test-domain", "slo_app": "test_app", "slo_class": "critical"}],
   ["POST:/api/v1/graphQL?operationName=adsList", {"slo_domain": "test-domain", "slo_app": "test_app", "slo_class": "critical"}],
]
}
```


### Jeden endpoint muze spadat do vice class
Aktualne nebudeme resit, pripadne muze fe-api pridat do paramu `?operationName=blah&severity=critical`

Pokud by to tak neslo, tak budem muset pridat vahy ale tomu bcyh se osobne nejradsi vyhnul.
```json
{
"exact": {
   "POST:/api/v1/graphQL?operationName=adsList": { {"slo_domain": "test-domain", "slo_app": "test_app", "slo_class": "critical"}: 0}
   "POST:/api/v1/graphQL?operationName=adsList": { {"slo_domain": "test-domain", "slo_app": "test_app", "slo_class": "low"}: 100 , {"slo_domain": "test-domain", "slo_app": "test_app", "slo_class": "critical"}: 200 }
}
}
```

## Cache statistickyho classifieru
Bude drzet procentualni sanci pro kazdou kombinaci slo_ dat, na zaklade toho klasifikuje prichozi event.

- statistickej posledni fallback kterej klasifikuje uplne vsehcny
- musi vystavovat metriku s labelama te klasifikace kolik jich klasifikoval, musime sledovat!
- mel by statistiky pocitat nad nejakym plovouvcim oknem.
```
{"slo_domain": "test-domain", "slo_app": "test_app", "slo_class": "critical"}: 0.2
{"slo_domain": "test-domain", "slo_app": "test_app", "slo_class": "warning"}: 0.8
```