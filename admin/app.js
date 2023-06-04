class Queryresult {
    constructor() {
        this.reset()
    }
    reset() {
        this.pos = 0
        this.total = 0
        this.limit = 20
        this.items = []
        this.count = 0
    }
    attach(data, current) {
        this.pos = data.pos || 0
        this.total = data.total || 0
        this.limit = data.limit || 20
        let items = data.items || []
        this.count = items.length
        if (this.count && this.pos == 0) {
            this.pos = 1
        }

        // build items for table view
        this.items = items.map(item => {
            let row = []
            current.shows.forEach(field => {
                row.push({
                    value: item[field.name],
                    primary: field.primary,
                    selected: false,
                })
            })
            return row
        })
    }
}
class AdminObject {
    constructor(meta) {
        this.meta = meta
        this.name = meta.name
        this.path = meta.path
        this.group = meta.group
        this.primaryKey = meta.primaryKey
        this.pluralName = meta.pluralName
        let shows = []
        if (meta.shows) {
            shows = meta.shows.map(f => {
                return { name: f }
            })
        } else {
            shows = meta.fields
        }
        this.shows = shows.map(f => {
            return {
                headerName: f.name.toUpperCase().replace(/_/g, ' '),
                name: f.name,
                primary: f.name === meta.primaryKey,
            }
        })
    }
    get showsearch() {
        return (this.meta.searchables || []).length > 0
    }
}

const adminapp = () => ({
    loading: true,
    switching: false,
    site: {},
    navmenus: [],
    async init() {
        Alpine.store('queryresult', new Queryresult())
        Alpine.store('current', {})
        this.$router.config({ mode: 'hash', base: '/admin/' })
        let resp = await fetch('./admin.json', {
            method: 'POST',
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
        this.loading = false
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

        let elm = document.getElementById('objectcontent')
        if (elm) elm.innerHTML = ''

        this.$store.queryresult.reset()
        this.switching = true
        this.$store.current = obj
        this.$store.current.active = true

        this.$router.push(obj.path)

        let listpage = obj.listpage || './list.html'
        fetch(listpage).then(resp => {
            resp.text().then(text => {
                document.getElementById('objectcontent').innerHTML = text
                this.refreshcurrent()
                this.switching = false
            })
        })
    },
    queryprev(event) {
        if (event) {
            event.preventDefault()
        }
    },

    querynext(event) {
        if (event) {
            event.preventDefault()
        }
    },

    selectAllResult(event) {
        this.$store.queryresult.items.forEach(row => {
            row.selected = !row.selected
        })
    },
    refreshcurrent() {
        let query = {}
        let path = this.$store.current.path
        if (path[-1] != '/') {
            path += '/'
        }
        fetch(path, {
            method: 'POST',
            body: JSON.stringify(query),
        }).then(resp => {
            resp.json().then(data => {
                this.$store.queryresult.attach(data, this.$store.current)
            })
        })
    },
})