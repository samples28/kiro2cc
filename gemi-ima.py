import google.generativeai as genai
from PIL import Image
from io import BytesIO
import os

# IMPORTANT: Set your Gemini API key.
# You can get one from Google AI Studio.
# It's recommended to set it as an environment variable for security.
# Example: export GEMINI_API_KEY="YOUR_API_KEY"
# If you don't set the environment variable, replace "YOUR_API_KEY" below.
api_key = os.environ.get("GEMINI_API_KEY", "YOUR_API_KEY")
if api_key == "YOUR_API_KEY":
    print("API key not found in environment variables. Make sure to set GEMINI_API_KEY.")
    print("Using a placeholder key. The script will likely fail.")

genai.configure(api_key=api_key)


prompt = (
    "Create a picture of a nano banana dish in a fancy restaurant with a Gemini theme"
)

# The model name was taken from the original script.
# If "gemini-2.5-flash-image-preview" is not available, you might need to change it.
model = genai.GenerativeModel(model_name="gemini-2.5-flash-image-preview")

response = model.generate_content(
    contents=[prompt],
)

for part in response.candidates[0].content.parts:
    if part.text is not None:
        print(part.text)
    elif part.inline_data is not None:
        image = Image.open(BytesIO(part.inline_data.data))
        image.save("generated_image.png")
