const adminapp = (admin) => ({
    ...admin,
    $delimiters: ['[[', ']]'], // default delimiters is confusing with django template tag
    error: '',
    get navmenu() {
        let menus = []
        admin.objects.forEach(obj => {
            let group = menus.find(menu => menu.name == obj.group)
            if (group == undefined) {
                group = {
                    name: obj.group,
                    items: []
                }
                menus.push(group)
            }
            let selected = (admin.current != undefined && admin.current.path == obj.path)
            group.items.push({ selected, name: obj.pluralName, path: `${admin.prefix}${obj.path}/` })
        })
        return menus
    },

    get currentpath() {
        if (this.current == undefined) {
            return this.prefix
        }
        return `${this.prefix}${this.current.path}`
    },

    switchobject(objname) {
        this.error = '';
        console.log('switch', objname)
    },

    async showedit(editsel) {
        let renderpage = `${this.currentpath}/_/render/edit.html?refer=${this.current.path}`;
        this.error = '';
        try {
            let req = await fetch(renderpage, {
                method: 'POST',
            })
            let html = await req.text()
            this.injectRemoteHTML(document.getElementById(editsel), html)
        } catch (e) {
            this.error = `Failed to load ${this.current.name} edit page, ${e.toString()}`
            console.error(this.error)
        }
    },

    injectRemoteHTML(elm, html) {
        elm.innerHTML = html;
        Array.from(elm.querySelectorAll("script")).forEach(oldScript => {
            const newScript = document.createElement("script");
            Array.from(oldScript.attributes)
                .forEach(attr => newScript.setAttribute(attr.name, attr.value));
            newScript.appendChild(document.createTextNode(oldScript.innerHTML));
            oldScript.parentNode.replaceChild(newScript, oldScript);
        });
    }
})