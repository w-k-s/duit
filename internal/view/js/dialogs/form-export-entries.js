import {
	DialogForm
} from "./form.min.js"

import {
	i18n
} from "../i18n/i18n.min.js"

export function DialogExportEntries() {
	function isoDateString(d) {
		let date = String(d.getDate()).padStart(2, "0"),
			month = String(d.getMonth() + 1).padStart(2, "0"),
			year = d.getFullYear()

		return `${year}-${month}-${date}`
	}

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
			name: "fromDate",
			label: i18n("From"),
			type: "date",
			required: true
		}, {
			name: "toDate",
			label: i18n("To"),
			type: "date",
			required: true
		}]

		// Set default value
		var date = new Date(), year = date.getFullYear(), month = date.getMonth();
		var firstDayOfMonth = new Date(year, month, 1);
		var lastDayOfMonth = new Date(year, month + 1, 0);

		let defaultFromDate = defaultValue["fromDate"]
		if (defaultFromDate == null || defaultFromDate === "") {
			defaultValue["fromDate"] = isoDateString(firstDayOfMonth)
		}

		let defaultToDate = defaultValue["toDate"]
		if (defaultToDate == null || defaultToDate === "") {
			defaultValue["toDate"] = isoDateString(lastDayOfMonth)
		}

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