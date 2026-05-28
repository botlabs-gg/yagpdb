(function () {
    'use strict';

    var EDITOR_SELECTOR = '.template-editor';
    var STORAGE_KEY = 'cm-editor';

    function isEnabled() {
        return localStorage.getItem(STORAGE_KEY) !== 'false';
    }

    function sanitizeNbsp(line) {
        return line.replace(/\u00A0/g, ' ');
    }

    function alreadyUpgraded(textArea) {
        var next = textArea.nextSibling;
        return !!(next && next.classList && next.classList.contains('CodeMirror'));
    }


    function setup(textArea) {
        if (!textArea || !window.CodeMirror) return null;
        if (!isEnabled()) return null;
        if (alreadyUpgraded(textArea)) return textArea.nextSibling.CodeMirror;

        var codeMirror = CodeMirror.fromTextArea(textArea, {
            lineNumbers: true,
            mode: 'text/x-go',
            theme: 'ayu-mirage',
            lineWrapping: true,
            placeholder: textArea.getAttribute('placeholder') || '',
            viewportMargin: Infinity,
            electricChars: false,
            extraKeys: {
                'Ctrl-Space': 'autocomplete',
                'Alt-F': 'findPersistent',
            },
        });

        // Fix for annoying on mobile (U+00A0),
        codeMirror.on('beforeChange', function (cm, change) {
            if (!change.update) return;
            var needsFix = change.text.some(function (line) {
                return line.indexOf('\u00A0') !== -1;
            });
            if (needsFix) {
                change.update(change.from, change.to, change.text.map(sanitizeNbsp));
            }
        });

        codeMirror.on('change', function (cm) {
            cm.save();
            cm.getTextArea().dispatchEvent(new Event('input', { bubbles: true }));
        });

        return codeMirror;
    }

    function remove(wrapper) {
        if (wrapper && wrapper.CodeMirror) {
            wrapper.CodeMirror.toTextArea();
        }
    }

    function setupAll() {
        document.querySelectorAll(EDITOR_SELECTOR).forEach(setup);
    }

    function removeAll() {
        document.querySelectorAll('.CodeMirror').forEach(remove);
    }

    function syncCheckboxes() {
        document.querySelectorAll('.toggle-code-mirror').forEach(function (cb) {
            cb.checked = isEnabled();
        });
    }

    $(function () {
        syncCheckboxes();
        setupAll();

        $(document).on('change', '.toggle-code-mirror', function (e) {
            if (e.target.checked) {
                localStorage.setItem(STORAGE_KEY, 'true');
                setupAll();
            } else {
                localStorage.setItem(STORAGE_KEY, 'false');
                removeAll();
            }
            syncCheckboxes();
        });

        $(document).on('shown.bs.tab', function (e) {
            var href = $(e.target).attr('href');
            if (!href) return;
            $(href).find('.CodeMirror').each(function () {
                if (this.CodeMirror) {
                    this.CodeMirror.refresh();
                    applyDefaultHeight(this.CodeMirror);
                }
            });
        });
    });

    window.YAGCodeMirror = {
        setup: setup,
        remove: remove,
        setupAll: setupAll,
        removeAll: removeAll,
        isEnabled: isEnabled,
    };
})();
