Grafana Dashboard Ideas

I want one panel for each of those metrics:
- Successful EventStore.Append() Ops / sec
- Successful EventStore.Query() Ops / sec
- EventStore.Append() Ops Errors / sec
- EventStore.Query() Ops Errors / sec
- EventStore.Append() Ops Concurrency Conflicts / sec 
- SQL Insert Ops / sec
- SQL Select Ops / sec
- Average Successful EventStore.Append() Duration
- Average Successful EventStore.Query() Duration
- Canceled EventStore.Append() Ops / sec
- Canceled EventStore.Query() Ops / sec

One panel with all features / command types:
- Successful *command type* Ops / sec

One panel with all features / command types:
- *command type* Ops Errors / sec

One panel with all features / command types:
- *command type* Ops Concurrency Conflicts / sec 