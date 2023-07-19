async function parseResponseError(resp) {
    let text = undefined
    try {
        text = await resp.text()
        let data = JSON.parse(text)
        return data.error || text
    } catch (err) {
        return text || resp.statusText
    }
}

class ConfirmAction {
    constructor() {
        this.reset()
    }
    reset() {
        this.show = false
        this.action = {
            name: '',
            label: '',
            title: '',
            class: '',
            path: '',
            text: '',
            onDone: null,
            onFail: null,
        }
        this.keys = []
    }
    confirm({ action, keys }) {
        this.reset()
        this.action = Object.assign(this.action, action)
        this.keys = keys
        this.show = true
    }
    cancel(event) {
        if (event) {
            event.preventDefault()
        }
        this.show = false
        this.reset()
    }
}
class Toasts {
    constructor() {
        this.reset()
    }

    get class() {
        if (this.pending) {
            return 'bg-violet-50 border border-violet-200 text-sm text-violet-600 rounded-md p-4 w-64'
        }
        if (this.level === 'error') {
            return 'bg-orange-50 border border-orange-200 text-sm text-orange-600 rounded-md p-4'
        } else if (this.level === 'info') {
            return 'bg-blue-50 border border-blue-200 text-sm text-blue-600 rounded-md p-4'
        }
        return ''
    }
    reset() {
        this.show = false
        this.pending = false
        this.text = ''
        this.level = ''
    }
    info(text, timeout = 6000) {
        this.reset()
        this.text = text
        this.level = 'info'
        this.show = true
        setTimeout(() => {
            this.reset()
        }, timeout)
    }
    error(text, timeout = 10000) {
        this.reset()
        this.text = text
        this.level = 'error'
        this.show = true
        setTimeout(() => {
            this.reset()
        }, timeout)
    }
    doing(text) {
        this.reset()
        this.text = text
        this.pending = true
        this.show = true
    }
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
        this.orders = []
        this.filters = []
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

        if (current.prepareResult) {
            current.prepareResult(this.rows, this.total)
        }
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
        document.getElementById('btn_selectAll').checked = true
        this.selected = this.total
    }

    onSelectRow(event, row) {
        row.selected = !row.selected
        this.selected = this.rows.filter(row => row.selected).length
    }
    setFilters(filters) {
        this.filters.splice(0, this.filters.length)
        this.filters.push(...filters)
        return this
    }

    setOrders(orders) {
        this.orders.splice(0, this.orders.length)
        this.orders.push(...orders)
        return this
    }
    refresh() {
        let query = {
            keyword: this.keyword,
            pos: this.pos,
            limit: this.countPerPage,
            filters: this.filters,
            orders: this.orders
        }

        let current = Alpine.store('current')
        if (current.prepareQuery) {
            let q = current.prepareQuery(query)
            if (q) {
                query = q
            }
        }

        this.rows = []

        fetch(current.path, {
            method: 'POST',
            body: JSON.stringify(query),
        }).then(resp => {
            resp.json().then(data => {
                this.attach(data)
            })
        })
    }

    onDeleteOne(event) {
        Alpine.store('confirmAction').confirm({
            action: {
                method: 'DELETE',
                label: 'Delete',
                name: 'Delete',
                path: Alpine.store('current').path,
                class: 'text-white bg-red-500 hover:bg-red-700',
            },
            keys: [Alpine.store('editobj').primaryValue]
        })
    }

    doAction(event) {
        event.preventDefault()
        let { action, keys } = Alpine.store('confirmAction')

        Alpine.store('editobj').closeEdit()
        Alpine.store('confirmAction').cancel()

        Alpine.store('current').doAction(action, keys).then(() => {
            this.rows.forEach(row => {
                row.selected = false
            })
            this.selected = 0
            document.getElementById('btn_selectAll').checked = false
            Alpine.store('toasts').info(`${action.name} all records done`)
            this.refresh()
        }).catch(err => {
            Alpine.store('toasts').error(`${action.name} fail : ${err.toString()}`)
        })
    }
}
class EditObject {
    constructor({ mode, title, fields, names, primaryValue, row }) {
        this.mode = mode
        this.title = title
        this.fields = fields
        this.names = names
        this.primaryValue = primaryValue
        this.row = row
    }

    get api_url() {
        return Alpine.store('current').buildApiUrl(this.row)
    }

    async doSave(ev, closeWhenDone = true) {
        try {
            if (this.mode == 'create') {
                await Alpine.store('current').doCreate(this.fields)
            } else {
                await Alpine.store('current').doSave(this.primaryValue, this.fields.filter(f => f.dirty))
            }

            if (closeWhenDone) {
                this.closeEdit(ev)
            } else {
                this.mode = 'edit'
            }
            Alpine.store('queryresult').refresh()
            Alpine.store('toasts').info(`Save Done`)
        } catch (err) {
            Alpine.store('toasts').error(`Save Fail: ${err.toString()}`)
            this.closeEdit(ev)
        }
    }
    closeEdit(event, cancel = false) {
        this.mode = undefined
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
            return f
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

        this.filterables.forEach(f => {
            f.onSelect = this.onFilterSelect.bind(this)
        })

        let actions = meta.actions || []
        // check user can delete
        if (this.permissions.can_delete) {
            actions.push({
                method: 'DELETE',
                name: 'Delete',
                label: 'Delete',
                class: 'text-white bg-red-500 hover:bg-red-700',
            })
        }

        this.actions = actions.filter(action => !action.withoutObject).map(action => {
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
                Alpine.store('confirmAction').confirm({ action, keys })
            }
            if (!action.class) {
                action.class = 'bg-white text-gray-900 ring-1 ring-inset ring-gray-300 hover:bg-gray-50'
            }
            if (!action.label) {
                action.label = action.name
            }
            return action
        })
    }

    onFilterSelect(filter, value) {
        filter.selected = value || {}
        // refresh query
        let filters = this.filterables.filter(f => f.selected && f.selected.op).map(f => f.selected)
        Alpine.store('queryresult').setFilters(filters).refresh()
    }

    get hasFilterSelected() {
        return this.filterables.some(f => f.selected && f.selected.op)
    }
    get selectedFilters() {
        return this.filterables.filter(f => f.selected && f.selected.op)
    }

    getPrimaryValue(row) {
        let vals = {}
        this.primaryKey.forEach(key => {
            let f = this.fields.find(f => f.name === key)
            let v = row[key]
            if (v !== undefined) {
                if (f.foreign) {
                    vals[f.foreign.field] = v.value
                } else {
                    vals[key] = v
                }

            }
        })
        return vals
    }
    buildApiUrl(row) {
        if (!row) {
            return ''
        }
        let vals = ['api', this.name.toLowerCase()]
        this.primaryKey.forEach(key => {
            let f = this.fields.find(f => f.name === key)
            let v = row[key]
            if (v !== undefined) {
                if (f.foreign) {
                    v = v.value
                }
                vals.push(v)
            }
        })
        let config = Alpine.store('config')
        let api_host = config.api_host || location.origin
        if (!api_host.endsWith('/')) {
            api_host += '/'
        }
        return `${api_host}${vals.join('/')}`
    }
    get active() {
        return Alpine.store('current') === this
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
            throw new Error(await parseResponseError(resp))
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
            throw new Error(await parseResponseError(resp))
        }
        return await resp.json()
    }

    async doAction(action, keys) {
        for (let i = 0; i < keys.length; i++) {
            Alpine.store('toasts').doing(`${i + 1}/${keys.length}`)
            let params = new URLSearchParams(keys[i]).toString()
            let resp = await fetch(`${action.path}?${params}`, {
                method: action.method || 'POST',
            })
            if (resp.status != 200) {
                let reason = await parseResponseError(resp)
                Alpine.store('toasts').error(`${action.name} fail : ${reason}`)
                if (action.onFail) {
                    let result = await resp.text()
                    action.onFail(keys[i], result)
                }
                break
            }
            if (action.onDone) {
                let result = await resp.json()
                action.onDone(keys[i], result)
            }
        }
        Alpine.store('toasts').reset()
    }
}

const adminapp = () => ({
    site: {},
    navmenus: [],
    loadScripts: {},
    loadStyles: {},
    async init() {
        Alpine.store('toasts', new Toasts())
        Alpine.store('queryresult', new QueryResult())
        Alpine.store('current', {})
        Alpine.store('switching', false)
        Alpine.store('loading', true)
        Alpine.store('confirmAction', new ConfirmAction())
        Alpine.store('editobj', new EditObject({}))

        this.$router.config({ mode: 'hash', base: '/admin/' })
        let resp = await fetch('./admin.json', {
            method: 'POST',
            cache: "no-store",
        })
        let meta = await resp.json()
        this.site = meta.site
        let objects = meta.objects.map(obj => new AdminObject(obj))
        Alpine.store('objects', objects)
        Alpine.store('config', meta.site)

        if (meta.site.sitename) {
            document.title = `${meta.site.sitename}`
        }
        if (meta.site.slogan) {
            document.title = `${document.title} | ${meta.site.slogan}`
        }

        if (meta.site.favicon_url) {
            let link = document.createElement('link')
            link.rel = 'shortcut icon'
            link.href = meta.site.favicon_url
            document.head.appendChild(link)
        }

        this.user = meta.user
        this.user.name = this.user.firstName || this.user.email
        this.buildNavMenu()
        this.loadSidebar()
        this.loadAllScripts(objects)

        this.$store.loading = false
        this.onLoad()
    },

    loadAllScripts(objects) {
        objects.forEach(obj => {
            let scripts = obj.scripts || []
            scripts.forEach(s => {
                if (s.onload || this.loadScripts[s.src]) {
                    return
                }
                this.loadScripts[s.src] = true
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
        })
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
            // reset selected filters
            if (this.$store.current.filterables) {
                this.$store.current.filterables.forEach(f => {
                    f.selected = undefined
                })
            }
            if (this.$store.current === obj) return
        }
        this.closeEdit()

        this.$store.queryresult.reset()
        this.$store.switching = true
        this.$store.current = obj
        this.$router.push(obj.path)

        fetch(obj.listpage, {
            cache: "no-store",
        }).then(resp => {
            resp.text().then(text => {
                const elm = document.getElementById('query_content')
                if (!this.injectHtml(elm, text, obj)) {
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
            scripts.filter(s => s.onload).forEach(s => {
                hasOnload = true
                let sel = document.createElement('script')
                sel.src = s.src
                sel.defer = true
                document.head.appendChild(sel)
            })
        }
        elm.innerHTML = html
        return hasOnload
    },
    prepareEditobj(event, isCreate = false, row = undefined) {
        if (event) {
            event.preventDefault()
        }

        let names = {}
        let fields = this.$store.current.editables.map(editField => {
            let f = { ...editField }
            if (isCreate) {
                f.value = editField.defaultvalue()
            } else {
                f.value = row[editField.name]
            }
            names[editField.name] = f
            return f
        })

        let editobj = new EditObject(
            {
                mode: isCreate ? 'create' : 'edit',
                title: this.$store.current.editTitle || `${isCreate ? 'Add' : 'Edit'} ${this.$store.current.name}`,
                fields: fields,
                names,
                primaryValue: row ? row.primaryValue : undefined,
                row
            })

        let current = this.$store.current
        if (current.prepareEdit) {
            current.prepareEdit(editobj, isCreate, row)
        }

        fetch(current.editpage, {
            cache: "no-store",
        }).then(resp => {
            resp.text().then(text => {
                let elm = document.getElementById('edit_form')
                if (elm) {
                    this.$store.editobj = editobj
                    elm.innerHTML = text
                }
            })
        }).catch(err => {
            Alpine.store('toasts').error(`Load edit page fail: ${err.toString()}`)
        })
    },
    addObject(event) {
        this.prepareEditobj(event, true)
    },
    editObject(event, row) {
        this.prepareEditobj(event, false, row)
    },
    closeEdit(event, cancel = false) {
        if (event) {
            event.preventDefault()
        }

        let elm = document.getElementById('edit_form')
        if (elm) {
            elm.innerHTML = ''
        }
        if (this.$store.editobj) {
            this.$store.editobj.closeEdit(event, cancel)
        }
    },
})