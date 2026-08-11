// FOR HTTP GET OPERATIONS
// function performOperation(method, url) {
//     const xhr = new XMLHttpRequest();
//     xhr.open(method, url, true);
//     xhr.onreadystatechange = function() {
//         if (xhr.readyState === XMLHttpRequest.DONE) {
//             if (xhr.status === 200) {
//                 window.location.reload();
//             } else {
//                 console.error("Error:", xhr.status, xhr.statusText);
//             }
//         }
//     };
//     xhr.send();
// }
//
// document.addEventListener("DOMContentLoaded", function() {
//     document.querySelectorAll(".operation-button").forEach(button => {
//         button.addEventListener("click", function(event) {
//             event.preventDefault();
//             console.log("Button clicked");
//             const method = button.getAttribute("data-method");
//             const url = button.getAttribute("data-url");
//             performOperation(method, url);
//         });
//     });
// });
function performOperation(method, url, csrfToken) {
    const xhr = new XMLHttpRequest();
    xhr.open(method, url, true);
    xhr.setRequestHeader('X-CSRF-Token', csrfToken);
    xhr.onreadystatechange = function() {
        if (xhr.readyState === XMLHttpRequest.DONE) {
            if (xhr.status === 200) {
                console.log("Operation successful:", method, "for URL:", url);
                window.location.reload();
            } else {
                console.error("Error:", xhr.status, xhr.statusText);
            }
        }
    };

    xhr.send();
}


document.addEventListener("DOMContentLoaded", function() {
    document.querySelectorAll(".operation-button").forEach(button => {
        button.addEventListener("click", function(event) {
            event.preventDefault();
            const csrfToken = document.querySelector('input[name="csrf_token"]').value;
            const method = button.getAttribute("data-method");
            const url = button.getAttribute("data-url");
            performOperation(method, url, csrfToken);
        });
    });
});