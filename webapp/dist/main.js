/* 快速查詢 — Mattermost webapp 外掛（免編譯，直接使用 Mattermost 提供的 React）。 */
(function () {
    'use strict';

    var PLUGIN_ID = 'tw.com.cewolf.tiles';
    var React = window.React;
    var h = React.createElement;

    var COLORS = [
        {value: '', label: '預設'},
        {value: '#1D9E75', label: '綠'},
        {value: '#378ADD', label: '藍'},
        {value: '#D85A30', label: '橘'},
        {value: '#BA7517', label: '金'},
        {value: '#7F56D9', label: '紫'},
        {value: '#D24B4E', label: '紅'},
        {value: '#6B7280', label: '灰'},
    ];
    var ICONS = ['📘', '❓', '📄', '🛠️', '📦', '💡', '📊', '🔗', '📞', '🗂️'];

    // 表單上會顯示錯誤的欄位；伺服器回的錯誤不屬於這些欄位時，改顯示在表單底部
    var FORM_FIELDS = ['title', 'url', 'description', 'icon', 'color', 'channels'];

    var CSS = [
        '.cewolf-tiles{padding:12px 16px 24px;color:var(--center-channel-color);font-size:14px;}',
        '.cewolf-tiles__intro{margin:0 0 12px;color:rgba(var(--center-channel-color-rgb),.72);line-height:1.5;}',
        '.cewolf-tiles__bar{display:flex;gap:8px;margin-bottom:12px;}',
        '.cewolf-tiles__search{flex:1;min-width:0;height:36px;padding:0 12px;border-radius:4px;font-size:14px;',
        'border:1px solid rgba(var(--center-channel-color-rgb),.16);background:var(--center-channel-bg);color:var(--center-channel-color);}',
        '.cewolf-tiles__search:focus,.cewolf-tiles__input:focus{outline:none;border-color:var(--button-bg);box-shadow:inset 0 0 0 1px var(--button-bg);}',
        '.cewolf-tiles__refresh{width:36px;height:36px;border-radius:4px;border:1px solid rgba(var(--center-channel-color-rgb),.16);',
        'background:transparent;color:rgba(var(--center-channel-color-rgb),.64);cursor:pointer;font-size:16px;}',
        '.cewolf-tiles__refresh:hover{background:rgba(var(--center-channel-color-rgb),.08);}',
        '.cewolf-tiles__grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:10px;}',
        '.cewolf-tiles__tile{position:relative;display:flex;flex-direction:column;gap:6px;padding:14px 28px 14px 12px;border-radius:8px;',
        'min-height:104px;border:1px solid rgba(var(--center-channel-color-rgb),.12);border-top-width:4px;background:var(--center-channel-bg);',
        'cursor:pointer;user-select:none;transition:box-shadow .15s,transform .15s,background-color .15s,border-color .15s;}',
        // 整張 tile 是一顆按鈕：Mattermost 會把面板裡的連結染成藍色，這裡蓋回內文顏色，才不會看起來只有標題能點
        '.cewolf-tiles a.cewolf-tiles__tile,.cewolf-tiles a.cewolf-tiles__tile:hover,.cewolf-tiles a.cewolf-tiles__tile:focus,',
        '.cewolf-tiles a.cewolf-tiles__tile:visited{color:var(--center-channel-color) !important;text-decoration:none !important;}',
        '.cewolf-tiles__tile:hover{box-shadow:0 4px 12px rgba(0,0,0,.12);transform:translateY(-1px);',
        'background:rgba(var(--button-bg-rgb),.06);border-left-color:rgba(var(--button-bg-rgb),.4);border-right-color:rgba(var(--button-bg-rgb),.4);',
        'border-bottom-color:rgba(var(--button-bg-rgb),.4);}',
        '.cewolf-tiles__tile:active{transform:translateY(0);box-shadow:none;background:rgba(var(--button-bg-rgb),.12);}',
        '.cewolf-tiles__tile:focus-visible{outline:2px solid var(--button-bg);outline-offset:2px;}',
        '.cewolf-tiles__open{position:absolute;top:8px;right:6px;font-size:14px;color:rgba(var(--center-channel-color-rgb),.32);}',
        '.cewolf-tiles__tile:hover .cewolf-tiles__open{color:var(--button-bg);}',
        '.cewolf-tiles__icon{font-size:26px;line-height:1;}',
        '.cewolf-tiles__title{font-weight:600;line-height:1.3;}',
        '.cewolf-tiles__desc{font-size:12px;line-height:1.4;color:rgba(var(--center-channel-color-rgb),.64);}',
        '.cewolf-tiles__msg{padding:24px 8px;text-align:center;color:rgba(var(--center-channel-color-rgb),.64);}',
        '.cewolf-tiles__error{margin-bottom:12px;padding:10px 12px;border-radius:4px;font-size:13px;line-height:1.5;',
        'background:rgba(var(--error-text-color-rgb,210,75,78),.08);color:var(--error-text,#d24b4e);}',
        '.cewolf-tiles__notice{margin-bottom:12px;padding:10px 12px;border-radius:4px;font-size:13px;line-height:1.5;',
        'background:rgba(var(--online-indicator-rgb,6,214,160),.12);}',
        '.cewolf-tiles__hint{margin-top:16px;font-size:12px;color:rgba(var(--center-channel-color-rgb),.56);}',
        '.cewolf-tiles__hint code{font-size:12px;}',
        '.cewolf-tiles__section{display:flex;align-items:center;justify-content:space-between;gap:8px;margin:4px 0 10px;',
        'font-size:12px;font-weight:600;color:rgba(var(--center-channel-color-rgb),.64);}',
        '.cewolf-tiles__link{border:0;background:none;padding:0;color:var(--link-color);font-size:12px;font-weight:600;cursor:pointer;}',
        '.cewolf-tiles__btn{height:32px;padding:0 14px;border-radius:4px;border:0;font-size:13px;font-weight:600;cursor:pointer;',
        'background:var(--button-bg);color:var(--button-color);}',
        '.cewolf-tiles__btn--ghost{background:transparent;color:var(--button-bg);border:1px solid var(--button-bg);}',
        '.cewolf-tiles__btn:disabled{opacity:.5;cursor:default;}',
        '.cewolf-tiles__btn--block{width:100%;margin-bottom:12px;}',
        '.cewolf-tiles__row{display:flex;align-items:center;gap:6px;padding:8px;margin-bottom:6px;border-radius:6px;',
        'border:1px solid rgba(var(--center-channel-color-rgb),.12);border-left-width:4px;}',
        '.cewolf-tiles__row .cewolf-tiles__icon{font-size:20px;width:26px;text-align:center;}',
        '.cewolf-tiles__row-main{flex:1;min-width:0;}',
        '.cewolf-tiles__row-title,.cewolf-tiles__row-meta{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}',
        '.cewolf-tiles__row-title{font-weight:600;}',
        '.cewolf-tiles__row-meta{font-size:12px;color:rgba(var(--center-channel-color-rgb),.56);}',
        '.cewolf-tiles__icon-btn{flex:none;width:28px;height:28px;padding:0;border-radius:4px;border:0;background:transparent;',
        'color:rgba(var(--center-channel-color-rgb),.64);cursor:pointer;font-size:14px;}',
        '.cewolf-tiles__icon-btn:hover:not(:disabled){background:rgba(var(--center-channel-color-rgb),.08);}',
        '.cewolf-tiles__icon-btn:disabled{opacity:.3;cursor:default;}',
        '.cewolf-tiles__form{padding:14px;margin-bottom:16px;border-radius:8px;border:1px solid rgba(var(--center-channel-color-rgb),.16);}',
        '.cewolf-tiles__form h4{margin:0 0 12px;font-size:15px;font-weight:600;}',
        '.cewolf-tiles__field{margin-bottom:12px;}',
        '.cewolf-tiles__label{display:block;margin-bottom:4px;font-size:12px;font-weight:600;}',
        '.cewolf-tiles__input{width:100%;height:34px;padding:0 10px;border-radius:4px;font-size:14px;',
        'border:1px solid rgba(var(--center-channel-color-rgb),.16);background:var(--center-channel-bg);color:var(--center-channel-color);}',
        '.cewolf-tiles__field-error{margin-top:4px;font-size:12px;color:var(--error-text,#d24b4e);}',
        '.cewolf-tiles__picks{display:flex;flex-wrap:wrap;gap:6px;margin-top:6px;}',
        '.cewolf-tiles__pick{width:32px;height:32px;padding:0;border-radius:4px;border:2px solid transparent;cursor:pointer;',
        'font-size:16px;background:rgba(var(--center-channel-color-rgb),.06);}',
        '.cewolf-tiles__swatch{width:26px;height:26px;padding:0;border-radius:50%;border:2px solid var(--center-channel-bg);cursor:pointer;',
        'box-shadow:0 0 0 1px rgba(var(--center-channel-color-rgb),.16);}',
        '.cewolf-tiles__pick--on{border-color:var(--button-bg);}',
        '.cewolf-tiles__swatch--on{box-shadow:0 0 0 2px var(--button-bg);}',
        '.cewolf-tiles__actions{display:flex;gap:8px;justify-content:flex-end;}',
        '.cewolf-tiles__help{margin-bottom:6px;font-size:12px;line-height:1.5;color:rgba(var(--center-channel-color-rgb),.64);}',
        '.cewolf-tiles__chips{display:flex;flex-wrap:wrap;gap:4px;margin-bottom:6px;}',
        '.cewolf-tiles__chip{display:inline-flex;align-items:center;gap:2px;padding:2px 2px 2px 8px;border-radius:12px;font-size:12px;',
        'background:rgba(var(--button-bg-rgb),.08);color:var(--button-bg);}',
        '.cewolf-tiles__chip button{width:20px;height:20px;padding:0;border:0;border-radius:50%;background:transparent;color:inherit;cursor:pointer;}',
        '.cewolf-tiles__chip--missing{background:rgba(var(--error-text-color-rgb,210,75,78),.08);color:var(--error-text,#d24b4e);}',
        '.cewolf-tiles__checklist{max-height:180px;overflow-y:auto;margin-top:6px;border-radius:4px;',
        'border:1px solid rgba(var(--center-channel-color-rgb),.12);}',
        '.cewolf-tiles__check{display:flex;align-items:center;gap:8px;margin:0;padding:6px 10px;font-weight:normal;cursor:pointer;}',
        '.cewolf-tiles__check:hover{background:rgba(var(--center-channel-color-rgb),.04);}',
        '.cewolf-tiles__check input{margin:0;}',
        '.cewolf-tiles__check-team{margin-left:auto;font-size:12px;color:rgba(var(--center-channel-color-rgb),.56);}',
        '.cewolf-tiles__personal{margin-top:20px;}',
        '.cewolf-tiles__mobile{margin-top:24px;padding-top:16px;border-top:1px solid rgba(var(--center-channel-color-rgb),.12);}',
        '.cewolf-tiles__mobile .cewolf-tiles__hint{margin:4px 0 10px;line-height:1.6;}',
        '.cewolf-tiles__pin-result{margin-top:8px;font-size:12px;line-height:1.5;}',
        '.cewolf-tiles__add{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:6px;min-height:104px;padding:12px;',
        'border-radius:8px;border:1px dashed rgba(var(--center-channel-color-rgb),.32);background:transparent;',
        'color:rgba(var(--center-channel-color-rgb),.64);font-size:13px;cursor:pointer;}',
        '.cewolf-tiles__add:hover{border-color:var(--button-bg);color:var(--button-bg);}',
    ].join('');

    function injectStyles() {
        if (document.getElementById('cewolf-tiles-style')) {
            return;
        }
        var el = document.createElement('style');
        el.id = 'cewolf-tiles-style';
        el.textContent = CSS;
        document.head.appendChild(el);
    }

    function apiBase() {
        var base = window.basename || '';
        return base.replace(/\/$/, '') + '/plugins/' + PLUGIN_ID + '/api/v1';
    }

    // Mattermost 對外掛的非 GET 請求會比對 CSRF token；token 放在 MMCSRF cookie，本來就設計成讓前端讀取
    function csrfToken() {
        var m = document.cookie.match(/(?:^|;\s*)MMCSRF=([^;]+)/);
        return m ? decodeURIComponent(m[1]) : '';
    }

    // request 回傳 Promise；失敗時 Error.message 是伺服器給的中文訊息，Error.field 是出錯的欄位
    function request(method, path, body) {
        var headers = {'X-Requested-With': 'XMLHttpRequest'};
        if (method !== 'GET') {
            headers['X-CSRF-Token'] = csrfToken();
        }
        var opts = {method: method, credentials: 'same-origin', headers: headers};
        if (body !== undefined) {
            headers['Content-Type'] = 'application/json';
            opts.body = JSON.stringify(body);
        }
        return fetch(apiBase() + path, opts).then(function (r) {
            return r.json().catch(function () {
                return {};
            }).then(function (data) {
                if (!r.ok) {
                    var err = new Error(data.error || ('連線失敗（HTTP ' + r.status + '）'));
                    err.field = data.field || '';
                    throw err;
                }
                return data;
            });
        });
    }

    // 說明文字在 System Console 裡可以用 Markdown 標題，右側面板只顯示純文字
    function plainIntro(s) {
        return (s || '').replace(/^#+\s*/gm, '').trim();
    }

    function matches(tile, words) {
        var hay = ((tile.title || '') + ' ' + (tile.description || '')).toLowerCase();
        for (var i = 0; i < words.length; i++) {
            if (hay.indexOf(words[i]) === -1) {
                return false;
            }
        }
        return true;
    }

    function TileCard(props) {
        var t = props.tile;
        return h('a', {
            className: 'cewolf-tiles__tile',
            href: t.url,
            target: '_blank',
            rel: 'noopener noreferrer',
            title: t.url,
            style: {borderTopColor: t.color || 'var(--button-bg)'},
        },
        h('i', {className: 'icon icon-open-in-new cewolf-tiles__open', 'aria-hidden': 'true'}),
        t.icon ? h('span', {className: 'cewolf-tiles__icon', 'aria-hidden': 'true'}, t.icon) : null,
        h('span', {className: 'cewolf-tiles__title'}, t.title),
        t.description ? h('span', {className: 'cewolf-tiles__desc'}, t.description) : null);
    }

    function iconButton(label, title, disabled, onClick) {
        return h('button', {
            type: 'button',
            className: 'cewolf-tiles__icon-btn',
            title: title,
            'aria-label': title,
            disabled: disabled,
            onClick: onClick,
        }, label);
    }

    // ManageList 是管理模式的清單：每列可上下移動、編輯、刪除
    function ManageList(props) {
        var tiles = props.tiles;
        if (!tiles.length) {
            return h('div', {className: 'cewolf-tiles__msg'}, props.emptyText);
        }
        return h('div', null, tiles.map(function (t, i) {
            return h('div', {
                key: t.id,
                className: 'cewolf-tiles__row',
                style: {borderLeftColor: t.color || 'var(--button-bg)'},
            },
            h('span', {className: 'cewolf-tiles__icon', 'aria-hidden': 'true'}, t.icon || '🔗'),
            h('div', {className: 'cewolf-tiles__row-main'},
                h('div', {className: 'cewolf-tiles__row-title', title: t.url}, t.title),
                h('div', {className: 'cewolf-tiles__row-meta'}, props.meta ? props.meta(t) : t.url)),
            iconButton('↑', '往前移', props.busy || i === 0, function () {
                props.onMove(t, -1);
            }),
            iconButton('↓', '往後移', props.busy || i === tiles.length - 1, function () {
                props.onMove(t, 1);
            }),
            iconButton('✎', '編輯「' + t.title + '」', props.busy, function () {
                props.onEdit(t);
            }),
            iconButton('🗑', '刪除「' + t.title + '」', props.busy, function () {
                props.onDelete(t);
            }));
        }));
    }

    function urlHost(url) {
        try {
            return new URL(url).host;
        } catch (e) {
            return url;
        }
    }

    function channelLabel(info) {
        if (info.missing) {
            return '（已刪除的頻道）';
        }
        return (info.private ? '🔒 ' : '# ') + info.display_name + (info.archived ? '（已封存）' : '');
    }

    // audienceText 是管理清單上「誰看得到」的摘要
    function audienceText(t) {
        if (!t.audience || !t.audience.length) {
            return '所有人看得到';
        }
        return '限 ' + t.audience.map(function (a) {
            return a.missing ? '（已刪除的頻道）' : a.display_name;
        }).join('、') + ' 的成員';
    }

    // AudiencePicker 挑選顯示對象：不勾＝所有人看得到；勾了只有那些頻道的成員看得到
    function AudiencePicker(props) {
        var fq = React.useState('');
        var filter = fq[0];
        var setFilter = fq[1];
        var selected = props.value;
        var options = props.options;

        function toggle(id) {
            props.onChange(selected.indexOf(id) === -1 ? selected.concat([id]) : selected.filter(function (x) {
                return x !== id;
            }));
        }

        var picker;
        if (!options) {
            picker = h('div', {className: 'cewolf-tiles__help'}, '載入頻道清單中…');
        } else {
            var word = filter.trim().toLowerCase();
            var shown = word ? options.filter(function (c) {
                return (c.display_name + ' ' + (c.team_name || '')).toLowerCase().indexOf(word) !== -1;
            }) : options;
            var teams = {};
            options.forEach(function (c) {
                teams[c.team_name || ''] = true;
            });
            // 只有一個團隊時不必顯示團隊名稱
            var multiTeam = Object.keys(teams).length > 1;
            picker = h(React.Fragment, null,
                h('input', {
                    className: 'cewolf-tiles__input',
                    type: 'search',
                    placeholder: '搜尋頻道',
                    'aria-label': '搜尋頻道',
                    value: filter,
                    onChange: function (ev) {
                        setFilter(ev.target.value);
                    },
                }),
                h('div', {className: 'cewolf-tiles__checklist', role: 'group', 'aria-label': '頻道清單'},
                    shown.length ? shown.map(function (c) {
                        return h('label', {key: c.id, className: 'cewolf-tiles__check'},
                            h('input', {
                                type: 'checkbox',
                                checked: selected.indexOf(c.id) !== -1,
                                onChange: function () {
                                    toggle(c.id);
                                },
                            }),
                            h('span', null, channelLabel(c)),
                            multiTeam && c.team_name ? h('span', {className: 'cewolf-tiles__check-team'}, c.team_name) : null);
                    }) : h('div', {className: 'cewolf-tiles__help', style: {margin: 0, padding: '6px 10px'}}, '找不到符合的頻道')));
        }

        return h('div', {className: 'cewolf-tiles__field'},
            h('span', {className: 'cewolf-tiles__label'}, '顯示對象'),
            h('div', {className: 'cewolf-tiles__help'},
                (selected.length ? '只有下列頻道的成員看得到。' : '目前所有人都看得到；勾選頻道後，只有這些頻道的成員看得到。') +
                '私人頻道只列出你有加入的。'),
            selected.length ? h('div', {className: 'cewolf-tiles__chips'}, selected.map(function (id) {
                var info = props.known[id] || {id: id, missing: true};
                return h('span', {key: id, className: 'cewolf-tiles__chip' + (info.missing ? ' cewolf-tiles__chip--missing' : '')},
                    channelLabel(info),
                    h('button', {
                        type: 'button',
                        'aria-label': '移除 ' + (info.display_name || '已刪除的頻道'),
                        onClick: function () {
                            toggle(id);
                        },
                    }, '×'));
            })) : null,
            picker,
            props.error);
    }

    function TileForm(props) {
        var init = props.initial || {};
        var v = React.useState({
            title: init.title || '',
            url: init.url || '',
            description: init.description || '',
            icon: init.icon || '',
            color: init.color || '',
            channels: init.channels || [],
        });
        var values = v[0];
        var setValues = v[1];
        var e = React.useState({field: '', message: ''});
        var error = e[0];
        var setError = e[1];
        var b = React.useState(false);
        var busy = b[0];
        var setBusy = b[1];

        function set(name, value) {
            setValues(function (prev) {
                var next = Object.assign({}, prev);
                next[name] = value;
                return next;
            });
        }

        function submit(ev) {
            ev.preventDefault();
            setBusy(true);
            setError({field: '', message: ''});
            // 成功時由上層關閉表單（元件會被移除），所以只在失敗時恢復按鈕
            props.onSubmit(values).catch(function (err) {
                setBusy(false);
                setError({field: err.field || '', message: err.message});
            });
        }

        function fieldError(name) {
            return error.field === name ? h('div', {className: 'cewolf-tiles__field-error', role: 'alert'}, error.message) : null;
        }

        function textField(name, label, attrs) {
            return h('div', {className: 'cewolf-tiles__field'},
                h('label', {className: 'cewolf-tiles__label', htmlFor: 'cewolf-tiles-' + name}, label),
                h('input', Object.assign({
                    id: 'cewolf-tiles-' + name,
                    className: 'cewolf-tiles__input',
                    value: values[name],
                    onChange: function (ev) {
                        set(name, ev.target.value);
                    },
                }, attrs)),
                fieldError(name));
        }

        var generalError = error.message && FORM_FIELDS.indexOf(error.field) === -1;

        // 頻道名稱對照：先放編輯中項目原本的顯示對象（含已刪除的頻道），再用最新的頻道清單覆蓋
        var known = {};
        (init.audience || []).forEach(function (a) {
            known[a.id] = a;
        });
        (props.channelOptions || []).forEach(function (c) {
            known[c.id] = c;
        });

        return h('form', {className: 'cewolf-tiles__form', onSubmit: submit},
            h('h4', null, props.heading),
            textField('title', '標題（必填）', {maxLength: 50, autoFocus: true}),
            textField('url', '網址（必填）', {inputMode: 'url', placeholder: 'https://', maxLength: 2000}),
            textField('description', '說明', {maxLength: 100, placeholder: '一句話說明，搜尋時也會比對'}),
            h('div', {className: 'cewolf-tiles__field'},
                h('label', {className: 'cewolf-tiles__label', htmlFor: 'cewolf-tiles-icon'}, '圖示'),
                h('input', {
                    id: 'cewolf-tiles-icon',
                    className: 'cewolf-tiles__input',
                    value: values.icon,
                    maxLength: 16,
                    placeholder: '貼上或點選一個 emoji',
                    onChange: function (ev) {
                        set('icon', ev.target.value);
                    },
                }),
                h('div', {className: 'cewolf-tiles__picks'}, ICONS.map(function (icon) {
                    return h('button', {
                        key: icon,
                        type: 'button',
                        className: 'cewolf-tiles__pick' + (values.icon === icon ? ' cewolf-tiles__pick--on' : ''),
                        'aria-label': '使用 ' + icon,
                        onClick: function () {
                            set('icon', icon);
                        },
                    }, icon);
                })),
                fieldError('icon')),
            h('div', {className: 'cewolf-tiles__field'},
                h('span', {className: 'cewolf-tiles__label'}, '顏色'),
                h('div', {className: 'cewolf-tiles__picks', role: 'radiogroup', 'aria-label': '顏色'}, COLORS.map(function (c) {
                    var on = values.color === c.value;
                    return h('button', {
                        key: c.value || 'default',
                        type: 'button',
                        role: 'radio',
                        'aria-checked': on,
                        'aria-label': c.label,
                        title: c.label,
                        className: 'cewolf-tiles__swatch' + (on ? ' cewolf-tiles__swatch--on' : ''),
                        style: {background: c.value || 'var(--button-bg)'},
                        onClick: function () {
                            set('color', c.value);
                        },
                    });
                })),
                fieldError('color')),
            // 只有共用項目有顯示對象；channelOptions 是 undefined 代表不顯示這個欄位，null 代表還在載入
            props.channelOptions !== undefined ? h(AudiencePicker, {
                options: props.channelOptions,
                value: values.channels,
                known: known,
                error: fieldError('channels'),
                onChange: function (next) {
                    set('channels', next);
                },
            }) : null,
            generalError ? h('div', {className: 'cewolf-tiles__error', role: 'alert'}, error.message) : null,
            h('div', {className: 'cewolf-tiles__actions'},
                h('button', {type: 'button', className: 'cewolf-tiles__btn cewolf-tiles__btn--ghost', disabled: busy, onClick: props.onCancel}, '取消'),
                h('button', {type: 'submit', className: 'cewolf-tiles__btn', disabled: busy}, busy ? '儲存中…' : '儲存')));
    }

    function TilesPanel() {
        var v = React.useState({loading: true, data: null, failed: false});
        var view = v[0];
        var setView = v[1];
        var q = React.useState('');
        var query = q[0];
        var setQuery = q[1];
        var m = React.useState('view'); // 'view'：一般畫面；'shared'：管理共用項目；'personal'：管理我的捷徑
        var mode = m[0];
        var setMode = m[1];
        var a = React.useState(null);
        var adminTiles = a[0];
        var setAdminTiles = a[1];
        var c = React.useState(null); // 顯示對象可選的頻道；null 代表還在載入
        var channelOptions = c[0];
        var setChannelOptions = c[1];
        // 表單：{scope: 'shared' | 'personal', tile: 要編輯的項目（新增時為 null）, fromView: 是否從一般畫面直接新增}
        var f = React.useState(null);
        var form = f[0];
        var setForm = f[1];
        var n = React.useState(null); // 操作結果：{kind: 'ok' | 'error', text}
        var notice = n[0];
        var setNotice = n[1];
        var bz = React.useState(false);
        var busy = bz[0];
        var setBusy = bz[1];
        var pr = React.useState(null); // 「加到我的最愛」的結果，顯示在按鈕下方：{kind, text}
        var pinResult = pr[0];
        var setPinResult = pr[1];

        var load = React.useCallback(function () {
            setView(function (prev) {
                return {loading: true, data: prev.data, failed: false};
            });
            return request('GET', '/tiles').then(function (data) {
                setView({loading: false, data: data, failed: false});
            }).catch(function () {
                setView(function (prev) {
                    return {loading: false, data: prev.data, failed: true};
                });
            });
        }, []);

        // loadAdmin 不會 reject：失敗時直接顯示錯誤並回傳 false，呼叫端據此決定要不要顯示成功訊息
        var loadAdmin = React.useCallback(function () {
            return request('GET', '/admin/tiles').then(function (data) {
                setAdminTiles(data.tiles || []);
                return true;
            }).catch(function (err) {
                setNotice({kind: 'error', text: err.message});
                return false;
            });
        }, []);

        React.useEffect(function () {
            load();
        }, [load]);

        // 把跟 bot 的私訊加到「我的最愛」：側邊欄分類存在伺服器上，手機 app 會同步看到
        function pin() {
            setBusy(true);
            setPinResult(null);
            request('POST', '/pin').then(function (data) {
                setBusy(false);
                setPinResult({kind: 'ok', text: data.already ?
                    '「快速查詢助手」已經在你的「我的最愛」裡了。' :
                    '已加到「我的最愛」，手機上打開 Mattermost 就看得到「快速查詢助手」。'});
            }).catch(function (err) {
                setBusy(false);
                setPinResult({kind: 'error', text: err.message});
            });
        }

        function setPersonal(tiles) {
            setView(function (prev) {
                return {loading: false, failed: false, data: Object.assign({}, prev.data, {personal: tiles})};
            });
        }

        function openShared() {
            setNotice(null);
            setForm(null);
            setAdminTiles(null);
            setChannelOptions(null);
            setMode('shared');
            loadAdmin();
            request('GET', '/admin/channels').then(function (data) {
                setChannelOptions(data.channels || []);
            }).catch(function (err) {
                setChannelOptions([]);
                setNotice({kind: 'error', text: '無法載入頻道清單：' + err.message});
            });
        }

        function openPersonal(withForm) {
            setNotice(null);
            setMode('personal');
            setForm(withForm ? {scope: 'personal', tile: null, fromView: true} : null);
        }

        function backToView(keepNotice) {
            setMode('view');
            setForm(null);
            if (!keepNotice) {
                setNotice(null);
            }
            // 先清掉舊資料，避免重新載入完成前閃過過時的畫面
            setView({loading: true, data: null, failed: false});
            return load();
        }

        function saveForm(values) {
            var current = form;
            var base = current.scope === 'shared' ? '/admin/tiles' : '/personal';
            var req = current.tile ?
                request('PUT', base + '/' + encodeURIComponent(current.tile.id), values) :
                request('POST', base, values);
            return req.then(function (saved) {
                var ok = {kind: 'ok', text: '已儲存「' + saved.title + '」'};
                setForm(null);
                if (current.fromView) {
                    // 從一般畫面按「新增捷徑」進來的，存好就直接回去
                    setNotice(ok);
                    return backToView(true);
                }
                // 等清單重新載入後才顯示成功訊息，避免訊息出現了清單卻還是舊的
                var reload = current.scope === 'shared' ? loadAdmin() : load().then(function () {
                    return true;
                });
                return reload.then(function (loaded) {
                    if (loaded) {
                        setNotice(ok);
                    }
                });
            });
        }

        // run 執行清單上的操作（移動、刪除），期間鎖住按鈕避免連點
        function run(promise, okText, apply) {
            setBusy(true);
            setNotice(null);
            return promise.then(function (data) {
                apply(data.tiles || []);
                setBusy(false);
                if (okText) {
                    setNotice({kind: 'ok', text: okText});
                }
            }).catch(function (err) {
                setBusy(false);
                setNotice({kind: 'error', text: err.message});
            });
        }

        // listProps 是兩種管理清單共用的操作；只差在 API 路徑與更新哪份資料
        function listProps(scope) {
            var base = scope === 'shared' ? '/admin/tiles/' : '/personal/';
            var apply = scope === 'shared' ? setAdminTiles : setPersonal;
            return {
                busy: busy,
                onMove: function (t, offset) {
                    run(request('POST', base + encodeURIComponent(t.id) + '/move', {offset: offset}), null, apply);
                },
                onEdit: function (t) {
                    setNotice(null);
                    setForm({scope: scope, tile: t});
                },
                onDelete: function (t) {
                    // eslint-disable-next-line no-alert
                    if (!window.confirm('確定要刪除「' + t.title + '」嗎？')) {
                        return;
                    }
                    run(request('DELETE', base + encodeURIComponent(t.id)), '已刪除「' + t.title + '」', apply);
                },
            };
        }

        var data = view.data;
        var isAdmin = Boolean(data && data.is_admin);
        var personal = (data && data.personal) || [];
        var maxPersonal = (data && data.max_personal) || 0;
        var canAddPersonal = personal.length < maxPersonal;
        var noticeEl = notice ? h('div', {
            className: notice.kind === 'error' ? 'cewolf-tiles__error' : 'cewolf-tiles__notice',
            role: 'status',
        }, notice.text) : null;

        function formEl(heading) {
            return h(TileForm, {
                key: form.tile ? form.tile.id : 'new',
                heading: heading,
                initial: form.tile,
                // 只有共用項目有顯示對象，個人捷徑不傳就不會出現這個欄位
                channelOptions: form.scope === 'shared' ? channelOptions : undefined,
                onSubmit: saveForm,
                onCancel: function () {
                    if (form.fromView) {
                        backToView(false);
                    } else {
                        setForm(null);
                    }
                },
            });
        }

        if (mode === 'shared') {
            return h('div', {className: 'cewolf-tiles'},
                h('div', {className: 'cewolf-tiles__section'},
                    h('span', null, '管理共用項目'),
                    h('button', {type: 'button', className: 'cewolf-tiles__link', onClick: function () {
                        backToView(false);
                    }}, '完成')),
                noticeEl,
                form ? formEl(form.tile ? '編輯共用項目' : '新增共用項目') : h('button', {
                    type: 'button',
                    className: 'cewolf-tiles__btn cewolf-tiles__btn--block',
                    onClick: function () {
                        setNotice(null);
                        setForm({scope: 'shared', tile: null});
                    },
                }, '＋ 新增共用項目'),
                adminTiles === null ? h('div', {className: 'cewolf-tiles__msg'}, '載入中…') : h(ManageList, Object.assign({
                    tiles: adminTiles,
                    emptyText: '還沒有任何共用項目，按上方按鈕新增。',
                    meta: function (t) {
                        return audienceText(t) + ' · ' + urlHost(t.url);
                    },
                }, listProps('shared'))));
        }

        if (mode === 'personal') {
            var addEl;
            if (form) {
                addEl = formEl(form.tile ? '編輯我的捷徑' : '新增我的捷徑');
            } else if (canAddPersonal) {
                addEl = h('button', {
                    type: 'button',
                    className: 'cewolf-tiles__btn cewolf-tiles__btn--block',
                    onClick: function () {
                        setNotice(null);
                        setForm({scope: 'personal', tile: null});
                    },
                }, '＋ 新增捷徑');
            } else {
                addEl = h('div', {className: 'cewolf-tiles__help'}, '已達上限 ' + maxPersonal + ' 個，刪掉一個才能再新增。');
            }
            return h('div', {className: 'cewolf-tiles'},
                h('div', {className: 'cewolf-tiles__section'},
                    h('span', null, '管理我的捷徑（只有你看得到）'),
                    h('button', {type: 'button', className: 'cewolf-tiles__link', onClick: function () {
                        backToView(false);
                    }}, '完成')),
                noticeEl,
                addEl,
                h(ManageList, Object.assign({
                    tiles: personal,
                    emptyText: '還沒有捷徑。',
                    meta: function (t) {
                        return urlHost(t.url);
                    },
                }, listProps('personal'))));
        }

        var words = query.toLowerCase().split(/\s+/).filter(Boolean);
        var filterWords = function (list) {
            return words.length ? list.filter(function (t) {
                return matches(t, words);
            }) : list;
        };
        var shared = (data && data.tiles) || [];
        var sharedShown = filterWords(shared);
        var personalShown = filterWords(personal);

        var body;
        if (!data && view.loading) {
            body = h('div', {className: 'cewolf-tiles__msg'}, '載入中…');
        } else if (!data && view.failed) {
            body = h('div', {className: 'cewolf-tiles__msg'}, '無法載入選單，請按右上角重新整理或稍後再試。');
        } else if (words.length && !sharedShown.length && !personalShown.length) {
            body = h('div', {className: 'cewolf-tiles__msg'}, '找不到符合「' + query + '」的項目');
        } else {
            var sharedPart = null;
            if (!shared.length) {
                sharedPart = h('div', {className: 'cewolf-tiles__msg'}, isAdmin ?
                    '還沒有共用項目，按右上角的鉛筆開始新增。' :
                    '尚未設定共用項目，請系統管理員新增。');
            } else if (sharedShown.length) {
                sharedPart = h('div', {className: 'cewolf-tiles__grid'}, sharedShown.map(function (t) {
                    return h(TileCard, {key: t.id, tile: t});
                }));
            }

            // 搜尋時只顯示有符合的區塊；「新增捷徑」卡片也只在沒搜尋時出現
            var personalPart = null;
            if (maxPersonal > 0 && (!words.length || personalShown.length)) {
                var cards = personalShown.map(function (t) {
                    return h(TileCard, {key: t.id, tile: t});
                });
                if (!words.length && canAddPersonal) {
                    cards.push(h('button', {
                        key: 'add',
                        type: 'button',
                        className: 'cewolf-tiles__add',
                        onClick: function () {
                            openPersonal(true);
                        },
                    }, h('span', {className: 'cewolf-tiles__icon', 'aria-hidden': 'true'}, '＋'), '新增捷徑'));
                }
                personalPart = h('div', {className: 'cewolf-tiles__personal'},
                    h('div', {className: 'cewolf-tiles__section'},
                        h('span', null, '我的捷徑（只有你看得到）'),
                        personal.length ? h('button', {
                            type: 'button',
                            className: 'cewolf-tiles__link',
                            onClick: function () {
                                openPersonal(false);
                            },
                        }, '管理') : null),
                    h('div', {className: 'cewolf-tiles__grid'}, cards));
            }
            body = h(React.Fragment, null, sharedPart, personalPart);
        }

        var intro = plainIntro(data && data.intro);
        var trigger = (data && data.trigger) || 'tiles';

        return h('div', {className: 'cewolf-tiles'},
            intro ? h('p', {className: 'cewolf-tiles__intro'}, intro) : null,
            noticeEl,
            h('div', {className: 'cewolf-tiles__bar'},
                h('input', {
                    className: 'cewolf-tiles__search',
                    type: 'search',
                    placeholder: '輸入關鍵字篩選',
                    'aria-label': '篩選項目',
                    value: query,
                    onChange: function (ev) {
                        setQuery(ev.target.value);
                    },
                }),
                h('button', {
                    className: 'cewolf-tiles__refresh',
                    type: 'button',
                    title: '重新整理',
                    'aria-label': '重新整理',
                    onClick: load,
                }, h('i', {className: 'icon icon-refresh', 'aria-hidden': 'true'})),
                isAdmin ? h('button', {
                    className: 'cewolf-tiles__refresh',
                    type: 'button',
                    title: '管理共用項目',
                    'aria-label': '管理共用項目',
                    onClick: openShared,
                }, h('i', {className: 'icon icon-pencil-outline', 'aria-hidden': 'true'})) : null),
            body,
            h('div', {className: 'cewolf-tiles__mobile'},
                h('div', {className: 'cewolf-tiles__section'}, h('span', null, '在手機上用')),
                h('div', {className: 'cewolf-tiles__hint'},
                    '手機 app 看不到這個面板。把「快速查詢助手」加到「我的最愛」，手機上一點就開，在私訊裡直接打關鍵字就能查；也可以在任何頻道輸入 ',
                    h('code', null, '/' + trigger), '。'),
                h('button', {
                    type: 'button',
                    className: 'cewolf-tiles__btn cewolf-tiles__btn--ghost',
                    disabled: busy,
                    onClick: pin,
                }, busy ? '處理中…' : '📌 加到我的最愛'),
                pinResult ? h('div', {
                    className: 'cewolf-tiles__pin-result',
                    role: 'status',
                    style: pinResult.kind === 'error' ? {color: 'var(--error-text,#d24b4e)'} : null,
                }, pinResult.text) : null));
    }

    function Plugin() {}

    Plugin.prototype.initialize = function (registry, store) {
        injectStyles();
        // 用位置參數呼叫，新舊版 Mattermost 都相容
        var rhs = registry.registerRightHandSidebarComponent(TilesPanel, '快速查詢');
        registry.registerChannelHeaderButtonAction(
            h('i', {className: 'icon icon-lightbulb-outline', style: {fontSize: '18px'}}),
            function () {
                store.dispatch(rhs.toggleRHSPlugin);
            },
            '快速查詢',
            '快速查詢'
        );
    };

    window.registerPlugin(PLUGIN_ID, new Plugin());
}());
