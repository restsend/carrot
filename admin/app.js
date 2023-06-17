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
    s = s.replace(/&/g, '&amp;')
    s = s.replace(/</g, '&lt;')
    s = s.replace(/>/g, '&gt;')
    return s
}

class Queryresult {
    constructor() {
        this.reset()
    }
    reset() {
        this.countPerPage = 20
        this.pos = 0
        this.total = 0
        this.limit = 20
        this.items = []
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
        this.rows = items.map(item => {
            item.primaryValue = current.getPrimaryValue(item)
            return item
        })

        this.items = items.map(item => {
            let row = []
            current.shows.forEach(field => {
                row.push({
                    value: field.format(item[field.name]),
                    name: field.name,
                    primary: field.primary,
                    selected: false,
                    show_class: field.show_class(item[field.name]),
                })
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
        this.items.forEach(row => {
            row.selected = !row.selected
        })
        this.selected = this.items.filter(row => row.selected).length
    }

    selectResult(event) {
        event.preventDefault()
        this.items.forEach(row => {
            row.selected = true
        })
        document.getElementById('btn_selectall').checked = true
        this.selected = this.total
    }

    onselect(event, row) {
        row.selected = !row.selected
        this.selected = this.items.filter(row => row.selected).length
    }

    refresh() {
        let query = {
            keyword: this.keyword,
            pos: this.pos,
            limit: this.countPerPage,
        }
        let path = Alpine.store('current').path
        if (path[-1] != '/') {
            path += '/'
        }
        fetch(path, {
            method: 'POST',
            body: JSON.stringify(query),
        }).then(resp => {
            resp.json().then(data => {
                this.attach(data)
            })
        })
    }

    ondelete_one(event) {
        Alpine.store('confirmaction', { action: { name: 'Delete', label: 'Delete' }, keys: [Alpine.store('editobj').primaryValue] })
    }

    doaction(event) {
        event.preventDefault()
        let { action, keys } = Alpine.store('confirmaction')

        Alpine.store('editobj', { mode: '' })
        Alpine.store('showedit', false)
        Alpine.store('confirmaction', {})

        Alpine.store('current').doaction(action, keys).then(() => {
            Alpine.store('doing', { pos: 0 })

            this.items.forEach(row => {
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

    cancelaction(event, row) {
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
        this.primaryKey = meta.primaryKey
        this.pluralName = meta.pluralName
        this.scripts = meta.scripts || []
        this.style = meta.style || []

        let fields = meta.fields || []
        let requireds = meta.requireds || []

        const fn_build_filed_htmltype = (f) => {
            let edit_class = ''
            let htmltype = ''
            switch (f.type) {
                case 'bool': {
                    htmltype = 'checkbox'
                    break
                }
                case 'uint':
                case 'int':
                case 'float':
                case 'datetime':
                case 'string':
                    htmltype = 'text'
                    break
                default: {
                    htmltype = 'json'
                }
            }

            let fsize = 0
            if (/size:(\d+)/.test(f.tag || '')) {
                fsize = parseInt(f.tag.match(/size:(\d+)/)[1])
            }

            if (fsize > 64) {
                edit_class = 'w-full'
            } else if (f.type === 'string' && fsize === 0) {
                htmltype = 'textarea'
            } else if (f.attribute && f.attribute.choices) {
                htmltype = 'select'
            }
            return { htmltype, edit_class: edit_class }
        }

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

            f.show_class = (value) => {
                if (typeof value === 'object') {
                    return 'w-72'
                }
                if (f.htmltype == 'text' || f.htmltype == 'json') {
                    if (typeof value === 'string' && value.length > 40) {
                        return 'w-72'
                    }
                }
            }

            const { htmltype, edit_class: edit_class } = fn_build_filed_htmltype(f)
            f.htmltype = htmltype
            f.edit_class = edit_class

            f.format = (value) => {
                if (f.attribute) {
                    let opt = f.attribute.choices.find(opt => opt.value == value)
                    if (opt) { return opt.label }
                }

                switch (f.type) {
                    case 'string':
                        return escapeHTML(value)
                    case 'bool': {
                        if (value === true) return IconYes
                        if (value === false) return IconNo
                        return IconUnknown
                    }
                    case 'datetime': {
                        if (!value) return ''
                        let d = new Date(value)
                        return d.toLocaleString()
                    }
                }

                if (value === null || value === undefined) {
                    return ''
                }
                // if value is object
                if (typeof value == 'object') {
                    return JSON.stringify(value)
                }

                return value.toString()
            }

            // convert value from string to type
            f.unmarshal = (value) => {
                if (value === null || value === undefined) {
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
                            console.log('unmarshal parse', f.type, value)
                            return JSON.parse(value)
                        }
                        console.log('unmarshal raw', f.type, value)
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
                path = `${path}/_/${action.path}`
            }
            action.path = path
            action.onclick = () => {
                let keys = []
                let queryresult = Alpine.store('queryresult')
                for (let i = 0; i < queryresult.items.length; i++) {
                    if (queryresult.items[i].selected) {
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

    get showsearch() {
        return this.searchables.length > 0
    }
    get showfilter() {
        return this.filterables.length > 0
    }

    async dosave(keys, vals) {
        let values = {}
        vals.forEach(v => {
            values[v.name] = v.unmarshal(v.value)
        })
        let params = new URLSearchParams(keys).toString()
        let resp = await fetch(`${this.path}/?${params}`, {
            method: 'PATCH',
            body: JSON.stringify(values),
        })
        if (resp.status != 200) {
            throw new Error(resp.statusText)
        }
        return await resp.json()
    }

    async docreate(vals) {
        let values = {}
        vals.forEach(v => {
            values[v.name] = v.unmarshal(v.value)
        })

        let resp = await fetch(`${this.path}/`, {
            method: 'PUT',
            body: JSON.stringify(values),
        })
        if (resp.status != 200) {
            throw new Error(resp.statusText)
        }
        return await resp.json()
    }

    async dodelete(keys) {
        for (let i = 0; i < keys.length; i++) {
            Alpine.store('doing', { pos: i + 1, total: keys.length })
            let params = new URLSearchParams(keys[i]).toString()
            let resp = await fetch(`${this.path}/?${params}`, {
                method: 'DELETE',
            })
            if (resp.status != 200) {
                Alpine.store('error', `Delete fail : ${err.toString()}`)
                break
            }
        }
    }

    async doaction(action, keys) {
        for (let i = 0; i < keys.length; i++) {
            Alpine.store('doing', { pos: i + 1, total: keys.length, action })
            let params = new URLSearchParams(keys[i]).toString()
            let resp = await fetch(`${action.path}/?${params}`, {
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
    loadscripts: {},

    async init() {
        Alpine.store('queryresult', new Queryresult())
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
        this.build_navmenu()

        if (this.$router.path) {
            // switch to current object
            let obj = this.$store.objects.find(obj => obj.path === this.$router.path)
            if (obj) {
                this.switchobject(null, obj)
            }
        }
        this.$store.loading = false
    },

    build_navmenu() {
        let menus = []
        this.$store.objects.forEach(obj => {
            let menu = menus.find(m => m.name === obj.group)
            if (!menu) {
                menu = { name: obj.group, items: [] }
                menus.push(menu)
            }
            menu.items.push(obj)
        });
        this.navmenus = menus
    },

    switchobject(event, obj) {
        if (event) {
            event.preventDefault()
        }

        if (this.$store.current) {
            if (this.$store.current === obj) return
            this.$store.current.active = false
        }

        let elm = document.getElementById('querycontent')
        if (elm) elm.innerHTML = ''

        this.$store.queryresult.reset()
        this.$store.switching = true
        this.$store.current = obj
        this.$store.current.active = true

        this.$router.push(obj.path)

        fetch(obj.listpage || './list.html', {
            cache: "no-store",
        }).then(resp => {
            resp.text().then(text => {
                let hasonload = this.injectHtml(this.$refs.querycontent, text, obj)
                if (!hasonload) {
                    this.$store.queryresult.refresh()
                }
                this.$store.switching = false
            })
        })
    },

    injectHtml(elm, html, obj) {
        elm.innerHTML = html
        let hasonload = false

        obj.scripts.forEach(s => {
            if (!s.onload && this.loadscripts[s.src]) {
                return
            }
            if (s.onload) {
                hasonload = true
            } else {
                this.loadscripts[s.src] = true
            }
            let scriptelm = document.createElement('script')
            scriptelm.src = s.src
            document.head.appendChild(scriptelm)
        })
        return hasonload
    },

    addobject(event) {
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
            docreate: (ev) => {
                // create row
                this.$store.current.docreate(this.$store.editobj.fields).then(() => {
                    this.closeedit(ev)
                    this.$store.queryresult.refresh()
                }).catch(err => {
                    this.closeedit(ev)
                })
            },
        }

        let obj = this.$store.current

        fetch(obj.editpage || './edit.html', {
            cache: "no-store",
        }).then(resp => {
            resp.text().then(text => {
                this.injectHtml(this.$refs.editcontent, text, obj)
            })
        }).catch(err => {
            this.$store.showedit = false
        })

    },
    editobject(event, row) {
        if (event) {
            event.preventDefault()
        }
        this.$store.showedit = true

        let fields = this.$store.current.editables.map(f => {
            let newf = { ...f }
            if (f.htmltype === 'json') {
                newf.value = JSON.stringify(row[f.name])
            } else {
                newf.value = row[f.name]
            }
            return newf
        })

        this.$store.editobj = {
            mode: 'edit',
            title: `Edit ${this.$store.current.name}`,
            fields: fields,
            primaryValue: row.primaryValue,
            dosave: (ev) => {
                // update row
                this.$store.current.dosave(this.$store.editobj.primaryValue, this.$store.editobj.fields).then(() => {
                    this.closeedit(ev)
                    this.$store.queryresult.refresh()
                }).catch(err => {
                    this.closeedit(ev)
                })
            },
        }

        let obj = this.$store.current

        fetch(obj.editpage || './edit.html', {
            cache: "no-store",
        }).then(resp => {
            resp.text().then(text => {
                this.injectHtml(this.$refs.editcontent, text, obj)
            })
        }).catch(err => {
            this.$store.showedit = false
        })

    },
    closeedit(event) {
        if (event) {
            event.preventDefault()
        }
        Alpine.store('showedit', false)
        Alpine.store('editobj', { mode: '' })
    },
})