/*
Package data is a PostgreSQL-backed checklist or "attempt list".

# Sample Use Case

Let's say you want to download a million items, and you suspect not all
items will get successfully downloaded on the first attempt.

Start with a list of the items to be downloaded.

	listName := "downloads"
	listItems := []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt", "f.txt"}
	AddBatch(ctx, PgPool, listName, listItems)

A worker can get a certain number of items to work on:

	// gets "a.txt", "b.txt", "c.txt"
	items, _ := GetBatch(ctx, PgPool, listName, "", 3)

For items that were unsuccessfully downloaded, the number of failed attempts
is incremented for that item. (A business rule can be set to abandon
downloading an item after a certain number of attempts.)

	count, _ := IncrementBatch(ctx, PgPool, ListName, []string{"a.txt", "c.txt"})

Items that were successfully downloaded can be removed from the list.

	count, _ := DeleteBatch(ctx, PgPool, ListName, []string{"b.txt"})

A worker can get more items from the list, starting past the last item in the
previously-worked-on batch:

	// gets "d.txt", "e.txt", "f.txt"
	items, _ := GetBatch(ctx, PgPool, listName, "c.txt", 3)

And the cycle can continue.
*/
package data
