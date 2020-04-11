import {
	DialogForm
} from "./form.min.js"

import {
	i18n
} from "../i18n/i18n.min.js"

export function DialogImportEntries() {
	
	function renderView(vnode) {
		// Parse attributes and set default value
		let title = vnode.attrs.title,
			message = vnode.attrs.message,
			loading = vnode.attrs.loading,
			defaultValue = vnode.attrs.defaultValue,
			onAccepted = vnode.attrs.onAccepted,
			onRejected = vnode.attrs.onRejected

		if (typeof title != "string") title = ""
		if (typeof message != "string") title = ""
		if (typeof loading != "boolean") loading = false
		if (typeof defaultValue != "object") defaultValue = {}
		if (typeof onAccepted != "function") onAccepted = () => { }
		if (typeof onRejected != "function") onRejected = () => { }

		// Create form fields
		let formFields = [{
			name: "import",
			label: i18n("Choose File"),
			type: "file",
			required: true
		}]

		formFields.forEach((field, i) => {
			let fieldName = field.name
			formFields[i].value = defaultValue[fieldName] || ""
		})

		// Render final dialog
		return m(DialogForm, {
			title: title,
			message: message,
			loading: loading,
			fields: formFields,
			onRejected: onRejected,
			onAccepted(data) {
				onAccepted(data)
			}
		})
	}

	return {
		view: renderView,
	}
}