I think there is no way around tracking some state in the load generator!

- it should know which books exist (5000 generated at start)
- it should know which readers exist (100 generated at start)
- it should know which books are currently lent to readers

Evolution of Books and Readers:

- books should eventually grow to max 10.000
- while growing, for each 50 added books one is removed
- once 10.000 books are reached, sometimes books are removed and shortly after replaced by another book
- Readers keep on registering, but the rate slows down the more it comes close to 50.000
- Once in a while a Reader cancels their contract, the rate depends a bit on the total number of Readers
- Readers borrow books, keep them for a while, and then return them
- Lending and returning should happen roughly 1000 times more often than adding/removing books
- Hm... Readers will typically borrow 2–4 books at once (sometimes more)
- Readers will typically return all currently lent books at once, sometimes a few less (need time to finish reading)

Additional ideas:

- We could have something like actors: Library Managers and Readers
- A Library Manager can add/remove books
- A Reader can borrow/return books
- New Readers can register
- Readers can cancel their contracts
- Let's only have one Library Manager
- The Library Manager will add new Book Copies to Circulation
- The Library Manager will remove Book Copies from Circulation (e.g., because they are damaged after many Lending/Returning)
- Readers will most of the time just return borrowed books, but sometimes (~5%) first query: BooksLentByReader
- Library Managers will sometimes query: BooksInCirculation to decide with books to add/remove
- Library Managers will sometimes query: BooksLentOut

The load generator, at startup, will have to do:

- query BooksInCirculation to decide if the Library already has enough books and to build its state
- query BooksLentOut to build its state
- query RegisteredReaders to build its state → we will have to implement this query(handler) first!