/** admin widgets
 * How to extend:
 *  1. add a new Widget
 *  2. set field.attribute.widget with the new Widget name
 *  3. add the new Widget to the window.AdminWidgets
 * */
const Icons = {
    Yes: `<span class="text-green-600"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-5 h-5">
<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
</svg></span>`,
    No: `<span class="text-red-600"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-5 h-5">
<path stroke-linecap="round" stroke-linejoin="round" d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
</svg></span>`,
    Unknown: `<span class="text-gray-600"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-5 h-5">
<path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9 5.25h.008v.008H12v-.008z" />
</svg></span>`
}

class BaseWidget {
    render(elm) {
        if (this.value) {
            this.renderWith(elm, this.value || '')
        }
    }

    renderWith(elm, text) {
        if (text && text.length > 40) {
            elm.classList.add('w-72', 'truncate')
        }
        elm.innerText = text
    }
    renderEditLabel(elm) {
        if (this.field.required) {
            let node = document.createElement('span')
            node.innerText = '*'
            node.className = 'text-red-600'
            elm.appendChild(node)
        }
        let node = document.createElement('span')
        node.innerText = this.field.label
        node.className = 'text-gray-700'
        elm.appendChild(node)
    }

    renderEdit(elm) {
        let node = document.createElement('input')
        node.type = 'text'
        node.className = 'block rounded-md border-0 py-1.5 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 sm:text-sm sm:leading-6'
        node.value = elm._x_model.get()
        node.placeholder = this.field.placeholder || ''
        node.autocomplete = 'off'
        elm.appendChild(node)

        node.addEventListener('change', (e) => {
            e.preventDefault()
            this.field.dirty = true
        })

        if (this.field.size >= 128) {
            node.classList.add('w-full')
        } else if (this.field.size > 64 && this.field.size < 128) {
            node.classList.add('w-96')
        } else if (this.field.size > 0 && this.field.size < 64) {
            node.classList.add('w-72')
        }
    }
}

class BooleanWidget extends BaseWidget {
    render(elm) {
        if (this.value === true) {
            elm.innerHTML = Icons.Yes
        } else if (this.value === false) {
            elm.innerHTML = Icons.No
        } else {
            elm.innerHTML = Icons.Unknown
        }
    }
    renderEditLabel(elm) {
        let node = document.createElement('input')
        node.type = 'checkbox'
        node.className = 'mr-2 h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-600'
        node.value = elm._x_model.get()
        node.addEventListener('change', (e) => {
            e.preventDefault()
            this.field.dirty = true
        })
        elm.appendChild(node)
        elm.className = 'mt-2 flex items-center'
        super.renderEditLabel(elm)
    }
    renderEdit(elm) {
        // ignore
    }
}
class TextareaWidget extends BaseWidget {
    renderEdit(elm) {
        let node = document.createElement('textarea')
        node.rows = 3
        node.className = 'block w-full rounded-md border-0 py-1.5 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 sm:text-sm sm:leading-6'
        node.value = elm._x_model.get()
        node.placeholder = this.field.placeholder || ''
        node.addEventListener('change', (e) => {
            e.preventDefault()
            this.field.dirty = true
        })
        elm.appendChild(node)
    }
}

class DateTimeWidget extends BaseWidget {
    render(elm) {
        if (this.value) {
            this.renderWith(elm, new Date(this.value).toLocaleString())
        }
    }
    renderEdit(elm) {
        let node = document.createElement('input')
        node.type = 'datetime-local'
        node.className = 'block rounded-md border-0 py-1.5 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 sm:text-sm sm:leading-6'
        if (elm._x_model.get()) {
            node.value = elm._x_model.get().substr(0, 16)
        }
        node.addEventListener('change', (e) => {
            e.preventDefault()
            let v = e.target.value
            if (v.length == 16) {
                v += ':00Z'
            }
            elm._x_model.set(new Date(v).toISOString())
            this.field.dirty = true
        })
        elm.appendChild(node)
    }
}

class StructWidget extends BaseWidget {
    render(elm) {
        if (this.value) {
            this.renderWith(elm, JSON.stringify(this.value))
        }
    }
    renderEdit(elm) {
        let node = document.createElement('textarea')
        node.rows = 5
        node.className = 'block w-full rounded-md border-0 py-1.5 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 sm:text-sm sm:leading-6'
        let v = elm._x_model.get()
        if (v) {
            node.value = JSON.stringify(v, null, 2)
        }
        node.addEventListener('change', (e) => {
            e.preventDefault()
            node.classList.remove('ring-red-600', 'ring-2')
            let v = e.target.value
            try {
                v = JSON.parse(v || '{}')
            } catch (e) {
                node.classList.add('ring-red-600', 'ring-2')
                return
            }
            elm._x_model.set(v)
            this.field.dirty = true
        })
        node.placeholder = this.field.placeholder || ''
        elm.appendChild(node)
    }
}

class ForeignKeyWidget extends BaseWidget {
    render(elm) {
        if (this.value) {
            this.renderWith(elm, this.value.label || this.value.value || '')
        }
    }

    async loadForeignValues() {
        let req = await fetch(this.field.foreign.path, {
            method: 'POST',
            body: JSON.stringify({
                foreign: true
            }),
        })
        let data = await req.json()
        if (!data.items) {
            return
        }
        return data.items
    }

    renderEdit(elm) {
        let node = document.createElement('select')
        node.className = 'block rounded-md border-0 py-1.5 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset  focus:ring-indigo-600 sm:text-sm sm:leading-6'
        let firstOption = document.createElement('option')
        firstOption.value = ''
        firstOption.disabled = true
        firstOption.innerText = this.field.placeholder || 'Select A Value'
        node.appendChild(firstOption)

        this.loadForeignValues().then(items => {
            if (!items) return

            if (!this.value) {
                this.value = items[0]
                elm._x_model.set(this.value.value)
                this.field.dirty = true
            }

            items.forEach(item => {
                let option = document.createElement('option')
                option.value = item.value
                option.innerText = item.label || item.value
                if (item.value == this.value.value) {
                    option.selected = true
                }
                node.appendChild(option)
            })
        })
        node.addEventListener('change', (e) => {
            e.preventDefault()
            this.value.value = e.target.value
            elm._x_model.set(this.value.value)
            this.field.dirty = true
        })
        elm.appendChild(node)
    }
}

class SelectWidget extends BaseWidget {
    render(elm) {
        if (this.value && this.field.attribute) {
            if (this.field.attribute.choices) {
                let opt = this.field.attribute.choices.find(opt => opt.value == this.value)
                if (opt) {
                    this.renderWith(elm, opt.label || opt.value)
                    return
                }
            }
            this.renderWith(elm, this.value)
        }
    }

    renderEdit(elm) {
        let node = document.createElement('select')
        node.className = 'block rounded-md border-0 py-1.5 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset  focus:ring-indigo-600 sm:text-sm sm:leading-6'
        let firstOption = document.createElement('option')
        firstOption.value = ''
        firstOption.disabled = true
        firstOption.innerText = this.field.placeholder || 'Select A Option'
        node.appendChild(firstOption)
        if (this.field.attribute && this.field.attribute.choices) {
            for (let opt of this.field.attribute.choices) {
                let option = document.createElement('option')
                option.value = opt.value
                option.innerText = opt.label || opt.value
                if (opt.value == this.value) {
                    option.selected = true
                }
                node.appendChild(option)
            }
        }
        node.addEventListener('change', (e) => {
            e.preventDefault()
            this.field.dirty = true
        })
        elm.appendChild(node)
    }
}

class PasswordWidget extends BaseWidget {
    render(elm) {
        if (this.value) {
            this.renderWith(elm, '********')
        }
    }

    renderEdit(elm) {
        let node = document.createElement('div')
        node.className = 'flex space-x-2'
        let btn = document.createElement('button')
        btn.className = 'inline-flex items-center px-2.5 py-2 border border-transparent text-xs font-medium rounded text-indigo-700 bg-indigo-100 hover:bg-indigo-200 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500'
        btn.innerText = 'Show Password Form'

        let input = document.createElement('input')
        input.type = 'text'
        input.autocomplete = 'off'
        input.className = 'hidden block w-64 rounded-md border-0 py-1.5 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 sm:text-sm sm:leading-6'
        input.placeholder = this.field.placeholder || 'Type password to change'
        input.autocomplete = 'off'
        btn.addEventListener('click', (e) => {
            e.preventDefault()
            input.classList.toggle('hidden')
            if (input.classList.contains('hidden')) {
                elm._x_model.set(this.field.oldValue) // don't change the value when hiding
                this.field.dirty = false
                btn.innerText = 'Show Password Form'
            } else {
                btn.innerText = 'Hide Password Form'
            }
        })

        input.addEventListener('change', (e) => {
            e.preventDefault()
            this.field.dirty = true
        })
        node.appendChild(btn)
        node.appendChild(input)
        elm.appendChild(node)
    }
}

let widgets = {
    'string': BaseWidget,
    'uint': BaseWidget,
    'int': BaseWidget,
    'float': BaseWidget,

    'bool': BooleanWidget,
    'textarea': TextareaWidget,
    'datetime': DateTimeWidget,
    'foreign': ForeignKeyWidget,
    'struct': StructWidget,
    'password': PasswordWidget,
    'select': SelectWidget,
}

window.AdminWidgets = widgets

function getWidget(field, value) {
    let widgetType = null
    if (field.foreign) {
        widgetType = 'foreign'
    } else {
        switch (field.type) {
            case 'string':
            case 'bool':
            case 'datetime':
            case 'uint':
            case 'int':
            case 'float':
                widgetType = field.type
                break
            default:
                widgetType = 'struct'
                break
        }
    }

    if (field.attribute) {
        if (field.attribute.widget) {
            widgetType = field.attribute.widget
        } else if (field.attribute.choices) {
            widgetType = 'select'
        }
    }

    if (field.tag && /size:(\d+)/.test(field.tag)) {
        field.size = parseInt(field.tag.match(/size:(\d+)/)[1])
    }

    if (widgetType == 'string' && field.size === undefined) {
        widgetType = 'textarea'
    }

    let widgetCls = window.AdminWidgets[widgetType]
    if (!widgetCls) {
        //
        console.warn(`Widget ${widgetType} not found, using struct widget`)
        widgetCls = window.AdminWidgets['struct']
    }
    let widget = new widgetCls()
    widget.field = field
    widget.value = value
    return widget
}

document.addEventListener('alpine:init', () => {
    Alpine.directive('admin-render', (el, { expression }, { evaluate }) => {
        let col = evaluate(expression)
        getWidget(col.field, col.value).render(el)
    })

    Alpine.directive('admin-edit-label', (el, { expression }, { evaluate }) => {
        let field = evaluate(expression)
        getWidget(field, field.value).renderEditLabel(el)
    })

    Alpine.directive('admin-edit', (el, { expression }, { evaluate }) => {
        let field = evaluate(expression)
        getWidget(field, field.value).renderEdit(el)
    })
})