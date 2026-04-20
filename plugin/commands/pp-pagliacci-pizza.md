---
description: "Printing Press CLI for Pagliacci Pizza. Order pizza, browse menus, manage rewards, and track deliveries from Pagliacci Pizza Capabilities include: access-device, address-info, address-name, analytics, customer, feedback, login, logout, menu-cache, menu-slices, menu-top, migrate-answer, migrate-question, order-list, order-list-item, order-list-pending, order-price, order-send, order-suggestion, password-forgot, password-reset, product-price, profile, quote-building, quote-store, register, reward-card, search, site-wide-message, store, stored-coupons, stored-credit, stored-gift, tail, time-window-days, time-windows, transfer-gift. Trigger phrases: 'install pagliacci-pizza', 'use pagliacci-pizza', 'run pagliacci-pizza', 'Pagliacci Pizza commands', 'setup pagliacci-pizza'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-pagliacci-pizza` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `pagliacci-pizza-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `pagliacci-pizza-pp-cli` command and execute.
