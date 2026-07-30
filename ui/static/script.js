const form = document.getElementById("shorten-form");
const resultBox = document.getElementById("shorten-result");
const output = document.getElementById("short-url-output");
const errorBox = document.getElementById("shorten-error");
const copyBtn = document.getElementById("copy-btn");

if (form) {
	form.addEventListener("submit", async (e) => {
		e.preventDefault();
		errorBox.classList.add("hidden");
		resultBox.classList.add("hidden");

		const longURL = document.getElementById("long-url-input").value;

		try {
			const res = await fetch("/shorten", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ longURL }),
			});

			const data = await res.json();

			if (!res.ok) {
				errorBox.textContent = data.message || "Something went wrong.";
				errorBox.classList.remove("hidden");
				return;
			}

			output.value = data;
			resultBox.classList.remove("hidden");
		} catch (err) {
			errorBox.textContent = "Network error, please try again.";
			errorBox.classList.remove("hidden");
		}
	});

	copyBtn.addEventListener("click", () => {
		navigator.clipboard.writeText(output.value);
		copyBtn.textContent = "Copied!";
		setTimeout(() => (copyBtn.textContent = "Copy"), 1500);
	});
}
