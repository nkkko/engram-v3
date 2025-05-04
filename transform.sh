#!/usr/bin/env bash
awk '/^#/{h=1;print;next} /^- \[ \]/{print;next} /^$/{print;next} /^##/{print;next}' todo.md > new_todo.md
echo "" >> new_todo.md
echo "# DONE" >> new_todo.md
grep '^- \[x\]' todo.md >> new_todo.md
