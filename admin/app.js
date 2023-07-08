const IconYes = `<span class="text-green-600"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-5 h-5">
<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
</svg></span>`
const IconNo = `<span class="text-red-600"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-5 h-5">
<path stroke-linecap="round" stroke-linejoin="round" d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
</svg></span>`
const IconUnknown = `<span class="text-gray-600"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-5 h-5">
<path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9 5.25h.008v.008H12v-.008z" />
</svg></span>`

function escapeHTML(s) {
    if (!s) {
        return ''
    }
    s = s.replace(/&/g, '&amp;')
    s = s.replace(/</g, '&lt;')
    s = s.replace(/>/g, '&gt;')
    return s
}

class QueryResult {
    constructor() {
        this.reset()
    }
    reset() {
        this.countPerPage = 20
        this.pos = 0
        this.total = 0
        this.limit = 20
        this.rows = []
        this.count = 0
        this.selected = 0
        this.keyword = ''
    }

    attach(data) {
        this.pos = data.pos || 0
        this.total = data.total || 0
        this.limit = data.limit || 20
        let items = data.items || []
        this.count = items.length

        let current = Alpine.store('current')
        this.rows = items.map(row => {
            row.primaryValue = current.getPrimaryValue(row)
            row.selected = false
            row.cols = current.shows.map(field => {
                return {
                    value: row[field.name],
                    field,
                    name: field.name,
                    primary: field.primary,
                }
            })
            return row
        })
    }

    get pos_value() {
        if (this.count == 0) { return 0 }
        return this.pos + 1
    }

    queryprev(event) {
        if (event) {
            event.preventDefault()
        }
        if (this.pos == 0) {
            return
        }
        this.pos = this.pos - this.countPerPage
        if (this.pos < 0) {
            this.pos = 0
        }
        this.refresh()
    }

    querynext(event) {
        if (event) {
            event.preventDefault()
        }
        let pos = this.pos + this.countPerPage
        if (pos >= this.total) {
            return
        }
        this.pos = pos
        this.refresh()
    }

    selectAll(event) {
        this.rows.forEach(row => {
            row.selected = !row.selected
        })
        this.selected = this.rows.filter(row => row.selected).length
    }

    selectResult(event) {
        event.preventDefault()
        this.rows.forEach(row => {
            row.selected = true
        })
        document.getElementById('btn_selectall').checked = true
        this.selected = this.total
    }

    onselect(event, row) {
        row.selected = !row.selected
        this.selected = this.rows.filter(row => row.selected).length
    }

    refresh() {
        let query = {
            keyword: this.keyword,
            pos: this.pos,
            limit: this.countPerPage,
        }
        let path = Alpine.store('current').path
        this.rows = []

        fetch(path, {
            method: 'POST',
            body: JSON.stringify(query),
        }).then(resp => {
            resp.json().then(data => {
                this.attach(data)
            })
        })
    }

    onDeleteOne(event) {
        Alpine.store('confirmaction', { action: { name: 'Delete', label: 'Delete' }, keys: [Alpine.store('editobj').primaryValue] })
    }

    doAction(event) {
        event.preventDefault()
        let { action, keys } = Alpine.store('confirmaction')

        Alpine.store('editobj', { mode: '' })
        Alpine.store('showedit', false)
        Alpine.store('confirmaction', {})

        Alpine.store('current').doAction(action, keys).then(() => {
            Alpine.store('doing', { pos: 0 })

            this.rows.forEach(row => {
                row.selected = false
            })
            this.selected = 0
            Alpine.store('info', `${action.name} all records done`)
            this.refresh()
        }).catch(err => {
            Alpine.store('doing', { pos: 0 })
            Alpine.store('error', `${action.name} fail : ${err.toString()}`)
        })
    }

    cancelAction(event, row) {
        event.preventDefault()
        Alpine.store('confirmaction', {})
    }
}

class AdminObject {
    constructor(meta) {
        this.permissions = meta.permissions || {}
        this.desc = meta.desc
        this.name = meta.name
        this.path = meta.path
        this.group = meta.group
        this.listpage = meta.listpage || 'list.html'
        this.editpage = meta.editpage || 'edit.html'
        this.primaryKey = meta.primaryKey
        this.pluralName = meta.pluralName
        this.scripts = meta.scripts || []
        this.styles = meta.styles || []
        this.icon = meta.icon
        this.invisible = meta.invisible || false
        let fields = meta.fields || []
        let requireds = meta.requireds || []


        this.fields = fields.map(f => {
            f.headerName = f.name.toUpperCase().replace(/_/g, ' ')
            f.primary = f.primary
            f.required = requireds.includes(f.name)

            f.defaultvalue = () => {
                switch (f.type) {
                    case 'bool': return false
                    case 'int': return 0
                    case 'uint': return 0
                    case 'float': return 0.0
                    case 'datetime': return ''
                    case 'string': return ''
                    default: return null
                }
            }
            // convert value from string to type
            f.unmarshal = (value) => {
                if (value === null || value === undefined) {
                    return value
                }

                if (f.foreign) {
                    return value
                }

                switch (f.type) {
                    case 'bool':
                        if (value === 'true') { return true }
                        return value
                    case 'uint':
                    case 'int': {
                        let v = parseInt(value)
                        if (isNaN(v)) { return undefined }
                        return v
                    }
                    case 'float': {
                        let v = parseFloat(value)
                        if (isNaN(v)) { return undefined }
                        return v
                    }
                    case 'datetime':
                    case 'string':
                        return value
                    default:
                        if (typeof value === 'string') {
                            return JSON.parse(value)
                        }
                        return value
                }
            }
        })

        let filter_fields = (names) => {
            return (names || []).map(name => {
                return fields.find(f => f.name === name)
            }).filter(f => f)
        }

        this.shows = filter_fields(meta.shows)
        this.editables = filter_fields(meta.editables)
        this.searchables = filter_fields(meta.searchables)
        this.filterables = filter_fields(meta.filterables)
        this.orderables = filter_fields(meta.orderables)

        let actions = meta.actions || []
        // check user can delete
        if (this.permissions.can_delete) {
            actions.push({
                method: 'DELETE',
                name: 'Delete',
                label: 'Delete',
                class: 'bg-red-500 hover:bg-red-700 text-white ',
            })
        }

        this.actions = actions.map(action => {
            let path = this.path
            if (action.path) {
                path = `${path}${action.path}`
            }
            action.path = path
            action.onclick = () => {
                let keys = []
                let queryresult = Alpine.store('queryresult')
                for (let i = 0; i < queryresult.rows.length; i++) {
                    if (queryresult.rows[i].selected) {
                        keys.push(queryresult.rows[i].primaryValue)
                    }
                }
                Alpine.store('confirmaction', { action: action, keys })
            }
            if (!action.class) {
                action.class = 'bg-white text-gray-900 ring-1 ring-inset ring-gray-300 hover:bg-gray-50'
            }
            return action
        })
    }

    getPrimaryValue(row) {
        let vals = {}
        this.primaryKey.forEach(key => {
            vals[key] = row[key]
        })
        return vals
    }

    get showSearch() {
        return this.searchables.length > 0
    }
    get showFilter() {
        return this.filterables.length > 0
    }

    async doSave(keys, vals) {
        let values = {}
        vals.forEach(v => {
            values[v.name] = v.unmarshal(v.value)
        })
        let params = new URLSearchParams(keys).toString()
        let resp = await fetch(`${this.path}?${params}`, {
            method: 'PATCH',
            body: JSON.stringify(values),
        })
        if (resp.status != 200) {
            throw new Error(resp.statusText)
        }
        return await resp.json()
    }

    async doCreate(vals) {
        let values = {}
        vals.forEach(v => {
            values[v.name] = v.unmarshal(v.value)
        })

        let resp = await fetch(`${this.path}`, {
            method: 'PUT',
            body: JSON.stringify(values),
        })
        if (resp.status != 200) {
            throw new Error(resp.statusText)
        }
        return await resp.json()
    }

    async doAction(action, keys) {
        for (let i = 0; i < keys.length; i++) {
            Alpine.store('doing', { pos: i + 1, total: keys.length, action })
            let params = new URLSearchParams(keys[i]).toString()
            let resp = await fetch(`${action.path}?${params}`, {
                method: action.method || 'POST',
            })
            if (resp.status != 200) {
                Alpine.store('error', `${action.name} fail : ${err.toString()}`)
                break
            }
        }
    }
}

const adminapp = () => ({
    site: {},
    navmenus: [],
    loadScripts: {},
    loadStyles: {},

    async init() {
        Alpine.store('queryresult', new QueryResult())
        Alpine.store('current', {})
        Alpine.store('showedit', false)
        Alpine.store('switching', false)
        Alpine.store('loading', true)
        Alpine.store('confirmaction', {})
        Alpine.store('doing', { pos: 0 })
        Alpine.store('error', '')
        Alpine.store('info', '')

        this.$router.config({ mode: 'hash', base: '/admin/' })
        let resp = await fetch('./admin.json', {
            method: 'POST',
            cache: "no-store",
        })
        let meta = await resp.json()
        this.site = meta.site
        let objects = meta.objects.map(obj => new AdminObject(obj))
        Alpine.store('objects', objects)

        this.user = meta.user
        this.user.name = this.user.firstName || this.user.email
        this.buildNavMenu()
        this.loadSidebar()

        this.$watch('$store.loading', val => {
            if (val === false) {
                this.onLoad()
            }
        })
        this.$store.loading = false
    },

    onLoad() {
        if (this.$router.path) {
            // switch to current object
            let obj = this.$store.objects.find(obj => obj.path === this.$router.path)
            if (obj) {
                this.switchObject(null, obj)
            }
        } else {
            if (this.site.dashboard) {
                fetch(this.site.dashboard, {
                    cache: "no-store",
                }).then(resp => {
                    this.$store.switching = true
                    resp.text().then(text => {
                        if (text) {
                            let elm = document.getElementById('query_content')
                            this.injectHtml(elm, text, null)
                        }
                        this.$store.switching = false
                    })
                })
            }
        }
    },
    loadSidebar() {
        fetch('sidebar.html', {
            cache: "no-store",
        }).then(resp => {
            resp.text().then(text => {
                if (text) {
                    this.injectHtml(this.$refs.sidebar, text, null)
                }
            })
        })
    },

    buildNavMenu() {
        let menus = []
        this.$store.objects.forEach(obj => {
            if (obj.invisible) { // skip invisible object
                return
            }
            let menu = menus.find(m => m.name === obj.group)
            if (!menu) {
                menu = { name: obj.group, items: [] }
                menus.push(menu)
            }
            menu.items.push(obj)
        });
        this.navmenus = menus
    },

    switchObject(event, obj) {
        if (event) {
            event.preventDefault()
        }

        if (this.$store.current) {
            if (this.$store.current === obj) return
            this.$store.current.active = false
        }

        let elm = document.getElementById('query_content')
        elm.innerHTML = ''
        this.closeEdit()

        this.$store.queryresult.reset()
        this.$store.switching = true
        this.$store.current = obj
        this.$store.current.active = true

        this.$router.push(obj.path)

        fetch(obj.listpage, {
            cache: "no-store",
        }).then(resp => {
            resp.text().then(text => {
                let hasOnload = this.injectHtml(elm, text, obj)
                if (!hasOnload) {
                    this.$store.queryresult.refresh()
                }
                this.$store.switching = false
            })
        })
    },

    injectHtml(elm, html, obj) {
        let hasOnload = false
        if (obj) {
            let scripts = obj.scripts || []
            scripts.forEach(s => {
                if (!s.onload && this.loadScripts[s.src]) {
                    return
                }
                if (s.onload) {
                    hasOnload = true
                } else {
                    this.loadScripts[s.src] = true
                }
                let sel = document.createElement('script')
                sel.src = s.src
                sel.defer = true
                document.head.appendChild(sel)
            })
            let styles = obj.styles || []
            styles.forEach(s => {
                if (this.loadStyles[s]) {
                    return
                }
                this.loadStyles[s] = true
                let sel = document.createElement('link')
                sel.rel = 'stylesheet'
                sel.type = 'text/css'
                sel.href = s
                document.head.appendChild(sel)
            })

        }
        elm.innerHTML = html
        return hasOnload
    },

    loadForeignValues(f, isCreate = false) {
        fetch(f.foreign.path, {
            method: 'POST',
            body: JSON.stringify({
                foreign: true
            }),
        }).then(resp => {
            resp.json().then(data => {
                if (!data.items) {
                    return
                }

                if (data.items.length > 0 && isCreate) {
                    f.value = data.items[0].value
                }
                f.values.push(...data.items)
            })
        })
    },

    addObject(event) {
        if (event) {
            event.preventDefault()
        }
        this.$store.showedit = true
        let fields = this.$store.current.editables.map(f => {
            let newf = { ...f }
            newf.value = f.defaultvalue()
            return newf
        })

        this.$store.editobj = {
            mode: 'create',
            title: `Add ${this.$store.current.name}`,
            fields: fields,
            doCreate: async (ev, closeWhenDone = true) => {
                // create row
                try {
                    await this.$store.current.doCreate(this.$store.editobj.fields)
                    if (closeWhenDone) {
                        this.closeEdit(ev)
                    }
                    this.$store.queryresult.refresh()
                } catch (err) {
                    console.error(err)
                    this.closeEdit(ev)
                }
            },
        }

        let obj = this.$store.current

        fetch(obj.editpage, {
            cache: "no-store",
        }).then(resp => {
            resp.text().then(text => {
                let elm = document.getElementById('edit_form')
                this.injectHtml(elm, text, obj)
            })
        }).catch(err => {
            this.$store.showedit = false
        })

    },
    editObject(event, row) {
        if (event) {
            event.preventDefault()
        }
        this.$store.showedit = true

        let fields = this.$store.current.editables.map(f => {
            let newf = { ...f }
            newf.dirty = false
            newf.value = row[f.name]// deep clone
            return newf
        })

        this.$store.editobj = {
            mode: 'edit',
            title: `Edit ${this.$store.current.name}`,
            fields: fields,
            primaryValue: row.primaryValue,
            doSave: async (ev, closeWhenDone = true) => {
                // update row
                try {
                    await this.$store.current.doSave(this.$store.editobj.primaryValue, this.$store.editobj.fields.filter(f => f.dirty))
                    if (closeWhenDone) {
                        this.closeEdit(ev)
                    }
                    this.$store.queryresult.refresh()
                } catch (err) {
                    console.error(err)
                    this.closeEdit(ev)
                }
            },
        }

        let obj = this.$store.current

        fetch(obj.editpage, {
            cache: "no-store",
        }).then(resp => {
            resp.text().then(text => {
                let elm = document.getElementById('edit_form')
                this.injectHtml(elm, text, obj)
            })
        }).catch(err => {
            this.$store.showedit = false
        })

    },
    closeEdit(event, cancel = false) {
        if (event) {
            event.preventDefault()
        }

        Alpine.store('showedit', false)
        Alpine.store('editobj', { mode: '' })
    },
})