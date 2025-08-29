import google.generativeai as genai
from PIL import Image
from io import BytesIO
import os
import argparse

def setup_api_key():
    """
    Sets up the Gemini API key from an environment variable.
    """
    api_key = os.environ.get("GEMINI_API_KEY")
    if not api_key:
        print("API key not found in environment variables. Make sure to set GEMINI_API_KEY.")
        print("Example: export GEMINI_API_KEY=\"YOUR_API_KEY\"")
        exit(1)
    genai.configure(api_key=api_key)

def generate_image(prompt: str, output_path: str):
    """
    Generates an image based on the prompt and saves it to the output path.
    """
    print(f"Generating image for prompt: '{prompt}'")
    
    # The model name was taken from the original script.
    # If "gemini-2.5-flash-image-preview" is not available, you might need to change it
    # to a valid image generation model.
    model = genai.GenerativeModel(model_name="gemini-2.5-flash-image-preview")

    try:
        response = model.generate_content(contents=[prompt])

        if not response.candidates:
            print("Error: The response did not contain any candidates.")
            return

        content = response.candidates[0].content
        image_part = None
        for part in content.parts:
            if part.inline_data is not None:
                image_part = part
                break
        
        if image_part:
            image_data = image_part.inline_data.data
            image = Image.open(BytesIO(image_data))
            image.save(output_path)
            print(f"Image saved successfully to '{output_path}'")
        else:
            print("Error: No image data found in the response.")
            for part in content.parts:
                if part.text is not None:
                    print(f"Received text part: {part.text}")

    except Exception as e:
        print(f"An error occurred during image generation: {e}")

def main():
    """
    Main function to parse arguments and generate the image.
    """
    parser = argparse.ArgumentParser(description="Generate images using the Gemini API.")
    parser.add_argument(
        "prompt",
        type=str,
        help="The text prompt for image generation."
    )
    parser.add_argument(
        "-o", "--output",
        type=str,
        default="generated_image.png",
        help="The path to save the generated image. (default: generated_image.png)"
    )
    args = parser.parse_args()

    setup_api_key()
    generate_image(args.prompt, args.output)

if __name__ == "__main__":
    main()
