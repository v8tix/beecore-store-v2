// PayPhone Integration JavaScript
// Note: PayPhone SDK (v1.1) generates deprecation warnings about unload event listeners
// This is a known issue with PayPhone's SDK and not caused by our implementation
let payphoneInitialized = false;
let selectedShippingAddress = null;

// Validates shipping address selection and triggers payment record creation
// Called when user selects/deselects a shipping address radio button
function validateShippingAddress() {
    const selectedAddress = document.querySelector('input[name="ShippingAddress"]:checked');
    selectedShippingAddress = selectedAddress ? selectedAddress.value : null;
    
    const statusDiv = document.getElementById('payphone-status');
    const ppButton = document.getElementById('pp-button');
    const instructionAlert = document.getElementById('payphone-instruction-alert');
    
    if (selectedShippingAddress && !payphoneInitialized) {
        // Hide the instruction alert when address is selected
        if (instructionAlert) {
            instructionAlert.style.display = 'none';
            instructionAlert.style.visibility = 'hidden';
            instructionAlert.classList.add('d-none');
        }
        if (statusDiv) {
            statusDiv.innerHTML = '<small class="text-info">✓ Shipping address selected. Creating payment record...</small>';
        }
        
        // Create payment record first, then initialize PayPhone after success
        if (typeof createPaymentRecord === 'function') {
            createPaymentRecord(selectedShippingAddress);
        } else {
            console.error('createPaymentRecord function does not exist!');
            initializePayPhone();
        }
    } else if (!selectedShippingAddress) {
        // Show the instruction alert when no address is selected
        if (instructionAlert) {
            instructionAlert.style.display = 'block';
            instructionAlert.style.visibility = 'visible';
            instructionAlert.classList.remove('d-none');
        }
        if (statusDiv) {
            statusDiv.innerHTML = '<small class="text-muted">PayPhone payment button will appear after selecting a shipping address</small>';
        }
        if (ppButton) {
            ppButton.innerHTML = '';
        }
        payphoneInitialized = false;
    }
}

// Creates payment record via AJAX before PayPhone initialization
// This ensures the payment record exists in the database before PayPhone redirect
// Parameters:
//   - shippingAddress: The selected shipping address ID from the radio button
function createPaymentRecord(shippingAddress) {
    const statusDiv = document.getElementById('payphone-status');
    
    try {
        // Get total amount from the page
        const totalElement = document.querySelector('.text-primary');
        const total = totalElement ? parseFloat(totalElement.textContent.replace('$', '')) : 0;
        
        // Get CSRF token for security
        const csrfTokenElement = document.querySelector('input[name="csrf_token"]');
        if (!csrfTokenElement) {
            throw new Error('CSRF token element not found');
        }
        const csrfToken = csrfTokenElement.value;
        
        fetch('/payphone/create-payment', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrfToken
            },
            body: JSON.stringify({
                shipping_address_id: shippingAddress,
                total: total
            })
        })
        .then(response => {
            if (!response.ok) {
                // Try to parse error response for detailed logging
                return response.json().then(errorData => {
                    console.error('Server error response:', {
                        status: response.status,
                        statusText: response.statusText,
                        errorData: errorData
                    });
                    throw new Error(`HTTP ${response.status}: ${errorData.message || response.statusText}`);
                }).catch(parseError => {
                    // If JSON parsing fails, log the parse error and throw original error
                    console.error('Failed to parse error response:', parseError);
                    throw new Error(`HTTP error! status: ${response.status} - ${response.statusText}`);
                });
            }
            return response.json();
        })
        .then(data => {
            if (statusDiv) {
                statusDiv.innerHTML = '<small class="text-success">✓ Payment record created. Initializing PayPhone...</small>';
            }
            // Now initialize PayPhone since payment record exists
            initializePayPhone();
        })
        .catch(error => {
            console.error('Failed to create payment record:', {
                error: error,
                message: error.message,
                stack: error.stack
            });
            
            // Provide user-friendly error message in the UI
            let displayMessage = error.message;
            
            // If it's a technical HTTP error, make it more user-friendly
            if (error.message.includes('HTTP error! status: 500')) {
                displayMessage = 'Payment service is temporarily unavailable. Please try again in a few moments.';
            } else if (error.message.includes('HTTP error! status: 4')) {
                displayMessage = 'There was an issue with your payment request. Please check your information and try again.';
            } else if (error.message.includes('HTTP error! status:')) {
                displayMessage = 'Payment service is currently unavailable. Please try again later.';
            }
            
            if (statusDiv) {
                statusDiv.innerHTML = `<small class="text-danger">⚠ ${displayMessage}</small>`;
            }
        });
        
    } catch (error) {
        console.error('Error in createPaymentRecord function:', error);
        if (statusDiv) {
            statusDiv.innerHTML = '<small class="text-danger">⚠ Error: ' + error.message + '</small>';
        }
    }
}

// Toggle payment method visibility - SIMPLIFIED FOR PAYPHONE ONLY
function togglePaymentMethod() {
    // PayPhone is now the only payment method, so always show PayPhone container
    const payphoneContainer = document.getElementById('payphone-container');
    const payphoneInstructions = document.getElementById('payphone-instructions');
    
    // Always show PayPhone elements since it's the only option
    if (payphoneContainer) {
        payphoneContainer.style.display = 'block';
    }
    if (payphoneInstructions) {
        payphoneInstructions.style.display = 'block';
    }
    
    // PayPhone initialization is now handled by validateShippingAddress() -> createPaymentRecord() -> initializePayPhone()
    // Don't initialize PayPhone directly here to avoid bypassing payment record creation
    
    /* COMMENTED OUT - PayPal logic for future reference
    const paypalSelected = document.getElementById('paypal').checked;
    const paypalContainer = document.getElementById('paypal-container');
    const traditionalSubmit = document.getElementById('traditional-submit');
    
    if (paypalSelected) {
        payphoneContainer.style.display = 'none';
        paypalContainer.style.display = 'block';
        traditionalSubmit.style.display = 'inline-block';
        payphoneInstructions.style.display = 'none';
    }
    */
}

// Initializes PayPhone payment button after payment record is created
// Requires: payment record must exist in database (created by createPaymentRecord)
// Requires: shipping address must be selected
// Requires: window.payphoneConfig must be set by the template
function initializePayPhone() {
    if (payphoneInitialized || !selectedShippingAddress) {
        return;
    }
    
    // Check if PayPhone configuration is available (set by template)
    if (typeof window.payphoneConfig === 'undefined') {
        console.error('PayPhone configuration not available');
        const statusDiv = document.getElementById('payphone-status');
        if (statusDiv) {
            statusDiv.innerHTML = '<small class="text-danger">⚠ PayPhone configuration error</small>';
        }
        return;
    }
    
    const config = window.payphoneConfig;
    
    const statusDiv = document.getElementById('payphone-status');
    if (statusDiv) {
        statusDiv.innerHTML = '<small class="text-info">Loading PayPhone payment button...</small>';
    }
    
    try {
        // Initialize PayPhone payment button with configuration from server
        new PPaymentButtonBox({
            token: config.token,
            clientTransactionId: config.clientTransactionId,
            amount: config.amount,
            amountWithoutTax: config.amountWithoutTax,
            amountWithTax: config.amountWithTax,
            tax: config.tax,
            service: config.service,
            tip: config.tip,
            currency: config.currency,
            storeId: config.storeId,
            reference: config.reference,
            lang: config.lang,
            defaultMethod: config.defaultMethod,
            timeZone: config.timeZone,
            lat: config.lat,
            lng: config.lng,
            optionalParameter: config.optionalParameter,
            responseUrl: config.responseUrl,
            cancelUrl: config.cancelUrl,
            environment: config.environment,
            phoneNumber: config.phoneNumber,
            email: config.email,
            documentId: config.documentId,
            identificationType: config.identificationType
        }).render('pp-button');
        
        if (statusDiv) {
            statusDiv.innerHTML = '<small class="text-success">✓ PayPhone ready! Click the button above to pay securely.</small>';
        }
        payphoneInitialized = true;
        
    } catch (error) {
        console.error('PayPhone initialization error:', error);
        if (statusDiv) {
            statusDiv.innerHTML = '<small class="text-danger">⚠ PayPhone initialization failed. Please try again.</small>';
        }
    }
}

// Initialize on page load
// Sets up initial state and validates any pre-selected shipping address
document.addEventListener('DOMContentLoaded', function() {
    // Check if shipping address is pre-selected
    validateShippingAddress();
    
    // Set initial payment method state (PayPhone only)
    togglePaymentMethod();
});
