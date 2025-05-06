#!/bin/bash

set -e  # Script abbrechen bei Fehlern

# Wo wollen wir es speichern?
TARGET_DIR="/usr/share/pomo/completion"
TARGET_FILE="$TARGET_DIR/pomo-completion.bash"
LINK_TARGET="/etc/bash_completion.d/pomo"

echo "🔧 Pomo Completion Setup gestartet..."

# 1. Ordner erstellen (falls nicht vorhanden)
echo "📁 Erstelle Zielordner: $TARGET_DIR (falls nötig)..."
sudo mkdir -p "$TARGET_DIR"

# 2. Completion-Skript erzeugen
echo "📝 Generiere Completion-Skript..."
./pomo completion bash | sudo tee "$TARGET_FILE" > /dev/null

# 3. Link im bash_completion.d setzen
if [ ! -e "$LINK_TARGET" ]; then
    echo "🔗 Erstelle symlink: $LINK_TARGET -> $TARGET_FILE"
    sudo ln -s "$TARGET_FILE" "$LINK_TARGET"
else
    echo "⚠️  Symlink existiert schon: $LINK_TARGET"
fi

echo "✅ Pomo Completion erfolgreich installiert."

# 4. Benutzerhinweis
echo ""
echo "ℹ️  Öffne ein neues Terminal oder führe folgendes aus, um Completion sofort zu aktivieren:"
echo ""
echo "    ubuntu/debian: source /etc/bash_completion"
echo "    arch:          source /usr/share/bash-completion/bash_completion"
echo ""
echo "Fertig! 🎉"
