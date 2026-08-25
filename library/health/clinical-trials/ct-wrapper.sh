#!/bin/bash
CLI=~/printing-press-library/library/health/clinical-trials/clinical-trials
QUERY="$*"
Q=$(echo "$QUERY" | tr '[:upper:]' '[:lower:]')

HU_STRIP='s/\b(aktív|kísérletek|kísérlet|toborzó|toborzás|kutatás|kutatások|betegség|vizsgálat|vizsgálatok|kérdés|kérem|mutasd|listázd|milyen|mik|hány)\b//gi'
HU_SMALL='s/\b(ki|kik|az|a|és|is|meg|mi|mit|hol|ahol)\b//gi'

if echo "$Q" | grep -qiE "hasonlít|össze|compare|vs |versus"; then
    STRIPPED=$(echo "$Q" | sed -E 's/(hasonlítsd|össze|compare|az|és|vs|versus|an|the|and|with)/ /gi' | tr -s ' ' | xargs)
    WORD1=$(echo "$STRIPPED" | awk '{print $1}')
    WORD2=$(echo "$STRIPPED" | awk '{print $NF}')
    echo ">>> compare \"$WORD1\" \"$WORD2\""
    $CLI compare "$WORD1" "$WORD2" --human-friendly
elif echo "$Q" | grep -qiE "phase.?3|fázis.?3|harmadik"; then
    TOPIC=$(echo "$Q" | sed -E 's/phase.?3|fázis.?3|harmadik fázis//gi' | xargs)
    echo ">>> phase3 \"$TOPIC\""
    $CLI phase3 "$TOPIC" --human-friendly
elif echo "$Q" | grep -qiE "trend|velocity|növeked|gyors|emerging|terjedés"; then
    TOPIC=$(echo "$Q" | sed -E 's/trend|velocity|növeked[^ ]*|gyors|emerging|terjedés//gi' | xargs)
    echo ">>> velocity \"$TOPIC\""
    $CLI velocity "$TOPIC" --human-friendly
elif echo "$Q" | grep -qiE "hotspot|ország|geograf|country|where|hol "; then
    TOPIC=$(echo "$Q" | sed -E 's/hotspot|ország|geograf|country|where|hol//gi' | xargs)
    echo ">>> hotspots \"$TOPIC\""
    $CLI hotspots "$TOPIC" --human-friendly
elif echo "$Q" | grep -qiE "sponsor|financ|fund|ki fizet|finanszír|who fund"; then
    TOPIC=$(echo "$Q" | sed -E 's/(sponsor|finanszír[^ ]*|financ[^ ]*|fund|ki fizet|who fund)//gi' | sed -E "$HU_SMALL" | tr -s ' ' | xargs)
    echo ">>> sponsors \"$TOPIC\""
    $CLI sponsors "$TOPIC" --human-friendly
elif echo "$Q" | grep -qiE "safety|mellékhatás|adverse|fda|side effect"; then
    TOPIC=$(echo "$Q" | sed -E 's/safety|mellékhatás|adverse|fda|side effect//gi' | xargs)
    echo ">>> safety \"$TOPIC\""
    $CLI safety "$TOPIC" --human-friendly
elif echo "$Q" | grep -qiE "risk|kockázat"; then
    TOPIC=$(echo "$Q" | sed -E 's/risk|kockázat//gi' | xargs)
    echo ">>> risk \"$TOPIC\""
    $CLI risk "$TOPIC" --human-friendly
elif echo "$Q" | grep -qiE "evidence|publication|publikáció|eredmény"; then
    TOPIC=$(echo "$Q" | sed -E 's/evidence|publication|publikáció|eredmény//gi' | xargs)
    echo ">>> evidence \"$TOPIC\""
    $CLI evidence "$TOPIC" --human-friendly
elif echo "$Q" | grep -qiE "watch|figyelés|monitor|track"; then
    TOPIC=$(echo "$Q" | sed -E 's/watch|figyelés|monitor|track//gi' | xargs)
    echo ">>> watch \"$TOPIC\""
    $CLI watch "$TOPIC" --human-friendly
elif echo "$Q" | grep -qiE "report|riport|összefoglaló|briefing"; then
    TOPIC=$(echo "$Q" | sed -E 's/report|riport|összefoglaló|briefing//gi' | xargs)
    echo ">>> report \"$TOPIC\""
    $CLI report "$TOPIC" --human-friendly
else
    TOPIC=$(echo "$Q" | sed -E "$HU_STRIP" | xargs)
    echo ">>> recruiting \"$TOPIC\""
    $CLI recruiting "$TOPIC" --human-friendly
fi
