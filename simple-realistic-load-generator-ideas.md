Let's not overcomplicate this, start as simple as possible, and iterate later!

What I want:

- The load generator MUST track internal state about:
  - Which Books are currently in circulation at all?
  - Which Readers are currently registered with un-canceled contracts?
  - Which Books are currently borrowed to which Readers?
- There should always be 950 - 1000 books in the library.
- There should always be between 9000 and 10000 registered readers.
- At startup, use the 3 available query handling features as they make sense to get that information.
  - then, if necessary, add books and readers as described above.
- Once the starting state is established, the following should happen:
  - take some inspiration from load_generator_ideas.md
  - the core principal should still be to have "a few" library managers and "many" readers acting realistically
  - but as stated above, don't overcomplicate this; we can iterate later!
  - to get some idempotent operations: just randomly repeat the same operation once (0.5% probability)
  - to get some error cases ... that's more complicated:
    - sometimes library managers miss the fact that a book was (just) lent out when they try to remove id (2% probability)
    - sometimes readers try to borrow a book copy that was (just) removed from circulation (not yet physically removed) (0.5% probability)

For now, create a completely new load generator and leave the old untouched so we can look good things up, like the worker pool impl.
Put the new one in a new directory:
- /example/simulation/cmd/ or so


