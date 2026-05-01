# VHS tape for ebay-pp-cli discovery flow demo
# Run with: vhs library/commerce/ebay/docs/discovery-demo.tape

Output library/commerce/ebay/docs/discovery-demo.gif

Set FontSize 14
Set Width 1000
Set Height 600
Set Theme "Dracula"
Set Padding 20
Set TypingSpeed 30ms

# 1. Show the killer feature: bid-filtered ending-soon auctions
Type "ebay-pp-cli auctions 'PSA Mariners Griffey' --has-bids --max-price 30 --per-page 5 --plain"
Sleep 500ms
Enter
Sleep 5s

# 2. Sold-comp pricing
Type "ebay-pp-cli comp 'Ken Griffey Jr 1989 Upper Deck PSA 9' --trim --plain"
Sleep 500ms
Enter
Sleep 5s

# 3. The just-fixed listings command
Type "ebay-pp-cli listings --nkw 'PSA Mariners 1980' --lh-bin 1 --udlo 10 --udhi 30 --plain"
Sleep 500ms
Enter
Sleep 5s
