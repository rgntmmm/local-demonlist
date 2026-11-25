# RELEASES

## Release Local Demonlist 1.0.0 (Nov 19, 2026)

- Added all the features from README

Enjoy the site!

## Update 1.0.1. What's new? (Nov 20, 2026)

### Cosmetic changes #1

- Updated footer icon (was replaced with Michi's icon)
- Updated level delete button (for admins)
- Added my contacts to footer (you can ask any questions about record submission, site dev etc.)
- Updated mobile version of the site
- Updated compact view of the demonlist (now it is actually compact)

## Update 1.0.2. What's new? (Nov 20, 2026)

### Cosmetic changes #2

- Small bugfixes

## Update 1.0.3. What's new? (Nov 22, 2026)

### Optimization changes #1

- Recent data is now stored in the cache for 20 minutes (even after session's termination)

## Update 1.0.4. What's new (Nov 25, 2026)

### Optimization changes #2

- Fixed the cache bug when user could not get fresh list updates because of the cache

Now the site checks if the new version of the list is available, and if it is, returns fresh data straight from database (and not from cache).
