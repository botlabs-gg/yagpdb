(function () {
    'use strict';

    var EDITOR_SELECTOR = '.template-editor';
    var STORAGE_KEY = 'cm-editor';
    var SCOPE_ATTR = 'data-cm-scope';

    // Editors inside a [data-cm-scope] element follow that scope's own toggle, falling back to the global setting.
    function scopeOf(el) {
        return el && el.closest ? el.closest('[' + SCOPE_ATTR + ']') : null;
    }

    function scopeKey(scope) {
        return STORAGE_KEY + ':' + scope.getAttribute(SCOPE_ATTR);
    }

    function isEnabled(el) {
        var scope = scopeOf(el);
        var value = scope ? localStorage.getItem(scopeKey(scope)) : null;
        if (value === null) value = localStorage.getItem(STORAGE_KEY);
        return value !== 'false';
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
        if (!isEnabled(textArea)) return null;
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
            cb.checked = isEnabled(cb);
        });
    }

    function toggleScope(scope, on) {
        localStorage.setItem(scopeKey(scope), on ? 'true' : 'false');
        scope.querySelectorAll(EDITOR_SELECTOR).forEach(function (textArea) {
            if (on) setup(textArea);
            else if (alreadyUpgraded(textArea)) remove(textArea.nextSibling);
        });
    }

    $(function () {
        syncCheckboxes();
        setupAll();

        $(document).on('change', '.toggle-code-mirror', function (e) {
            var scope = scopeOf(e.target);
            if (scope) {
                toggleScope(scope, e.target.checked);
                syncCheckboxes();
                return;
            }
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
            if (!href || href.charAt(0) !== '#') return;
            $(href).find('.CodeMirror').each(function () {
                if (this.CodeMirror) {
                    this.CodeMirror.refresh();
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
