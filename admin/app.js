const adminapp = () => ({
    loading: true,
    switching: false,
    site: {},
    objects: {},
    current: null,
    navmenus: [],
    qureyresult: {
        pos: 0,
        total: 0,
        limit: 20,
        items: [],
        count: 0,
    },
    async init() {
        this.$router.config({ mode: 'hash', base: '/admin/' })
        let resp = await fetch('./admin.json', {
            method: 'POST',
        })
        let meta = await resp.json()
        this.site = meta.site
        this.objects = meta.objects
        // convert objects.fileds.name to UPPER_CASE
        this.objects.forEach(obj => {
            if (!obj.shows) {
                obj.shows = obj.fields
            } else {
                obj.shows = obj.shows.map(f => {
                    return { name: f }
                })
            }

            obj.shows = obj.shows.map(f => {
                return {
                    headerName: f.name.toUpperCase().replace(/_/g, ' '),
                    name: f.name,
                    primary: f.name === obj.primaryKey,
                }
            })
        })

        this.user = meta.user
        this.user.name = this.user.firstName || this.user.email
        this.build_navmenu()

        if (this.$router.path) {
            // switch to current object
            let obj = this.objects.find(obj => obj.path === this.$router.path)
            if (obj) {
                this.switchobject(null, obj)
            }
        }
        this.loading = false
    },

    build_navmenu() {
        let menus = []
        this.objects.forEach(obj => {
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

        if (this.current) {
            if (this.current === obj) return
            this.current.active = false
        }

        let elm = document.getElementById('objectcontent')
        if (elm) elm.innerHTML = ''

        this.qureyresult = {}
        this.switching = true
        this.current = obj
        this.current.active = true

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
    refreshcurrent() {
        let query = {}
        let path = this.current.path
        if (path[-1] != '/') {
            path += '/'
        }
        fetch(path, {
            method: 'POST',
            body: JSON.stringify(query),
        }).then(resp => {
            resp.json().then(data => {

                this.qureyresult.pos = data.pos || 0
                this.qureyresult.total = data.total || 0
                this.qureyresult.limit = data.limit || 20
                let items = data.items || []
                this.qureyresult.count = items.length
                if (this.qureyresult.count && this.qureyresult.pos == 0) {
                    this.qureyresult.pos = 1
                }

                // build items for table view
                this.qureyresult.items = items.map(item => {
                    let row = []
                    this.current.shows.forEach(field => {
                        row.push({
                            value: item[field.name],
                            primary: field.primary,
                        })
                    })
                    return row
                })
            })
        })
    },
})